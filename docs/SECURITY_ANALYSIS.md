# Análisis y Mitigación de Seguridad en `webtyp.com/crud`

Este documento detalla el análisis de seguridad sobre el contrato de transporte de `webtyp.com/crud`, los vectores de ataque y vulnerabilidades identificados en el borde de entrada de las 4 operaciones estándar (`Create`, `Read`, `Update`, `Delete`), las mitigaciones arquitectónicas implementadas para blindar el framework y las pruebas automatizadas de verificación.

---

## 1. Resumen Ejecutivo y Filosofía de Seguridad

`webtyp.com/crud` es el punto de contacto entre el mundo exterior (red HTTP/RPC, clientes web WASM, agentes MCP) y la lógica de dominio de las aplicaciones. 

Bajo las reglas de seguridad por defecto de `webtyp`:
- **El servidor nunca confía en el cliente:** los datos del navegador pueden ser alterados o puenteados mediante llamadas directas (`curl`, scripts, agentes maliciosos).
- **El framework debe cerrar los agujeros antes de que toquen el dominio:** un módulo de negocio no debe tener que recordar programar defensas contra desbordamientos de memoria, IDs vacíos o inyecciones de campos inmutables; el montador CRUD debe garantizar que solo peticiones limpias y conformes al esquema lleguen a los callbacks del módulo.

---

## 2. Matriz de Amenazas y Vulnerabilidades Identificadas

| Vector | Amenaza / Clasificación | Componente Afectado | Impacto sin Mitigación |
|---|---|---|---|
| **V1** | **DoS por Lote Masivo No Acotado** *(CWE-400 / Uncontrolled Resource Consumption)* | `MountCreate`, `MountUpdate`, `MountDelete` (`create.go`, `update.go`, `delete.go`) | Un atacante envía arrays con miles de registros o IDs (`{"records":[…50000…]}`). El servidor asigna memoria sin control, bloquea la transacción de la BD o rompe el límite de variables SQL (`>999` en SQLite). |
| **V2** | **Mass Assignment & Modificación de Campos Inmutables** *(OWASP API3: Broken Object Property Level Authorization)* | `MountUpdate` (`update.go`) | El cliente envía `"fields": ["id", "tenant_id", "admin_role"]`. Si el módulo no valida manualmente, se puede mutar la clave primaria o reasignar el registro a otro inquilino (*cross-tenant data breach*). |
| **V3** | **Inyección de IDs Vacíos o Nulos** *(CWE-20 / Improper Input Validation)* | `MountUpdate`, `MountDelete` (`update.go`, `delete.go`, `op.go`) | El cliente envía `{"ids": ["", "  "]}`. Al tener longitud `1`, supera el chequeo `len == 0` y ejecuta `DELETE/UPDATE … WHERE id = ""` en la base de datos. |
| **V4** | **Omisión de Validación de Esquema (`Permitted`, `NotNull`)** *(CWE-20 / Input Validation)* | `MountCreate`, `MountUpdate` (`create.go`, `update.go`) | Si una petición proviene de fuera del navegador y el autor del módulo olvidó llamar a `rec.Validate()`, entran caracteres prohibidos, valores nulos requeridos o longitudes excesivas directo a la base de datos. |
| **V5** | **Saturación en Búsqueda (`Search`)** *(CWE-400 / Resource Consumption)* | `MountRead` (`read.go`) | Términos de búsqueda con megabytes de longitud en `ReadArgs.Search` provocan buffer inflation y saturación de CPU en consultas SQL `LIKE "%...%"`. |
| **V6** | **Inyección de Cargas Masivas en Campos Actualizados** *(CWE-20 / Input Validation)* | `MountUpdate` (`update.go`) | Se actualizan campos permitidos pero con valores corruptos o excesivos en `record` (por ejemplo, textos de 10 MB o caracteres no permitidos). |
| **V7** | **Inyección de IDs Duplicados** *(CWE-400 / Denial of Service)* | `MountUpdate`, `MountDelete` (`op.go`) | El cliente envía arrays con el mismo ID repetido cientos de veces (`["1", "1", …]`), saturando transacciones y bloqueos de fila en la BD. |
| **V8** | **Desbordamiento de Cursor `After` e Identificadores** *(CWE-400 / Resource Consumption)* | `MountRead`, `MountUpdate`, `MountDelete` (`read.go`, `op.go`) | El cliente envía strings gigantescos en el cursor de paginación (`After`) o en un ID individual. |

---

## 3. Mitigaciones Implementadas en `webtyp.com/crud`

### Mitigación V1: Techo Estricto por Lote (`MaxPage = 500`) y Protección contra Agotamiento de Memoria
- En `createIn.DecodeFields` (`create.go`) y `decodeStrings` (`op.go`) se verifica la longitud declarada del array JSON antes de instanciar o alojar los elementos:
  - Si `n > MaxPage` (`500`), no se ejecutan llamadas a `make([]string, n)` ni `c.list.Append()`. Se interrumpe la decodificación y se marca `tooMany = true`.
- `MountCreate`, `MountUpdate` y `MountDelete` rechazan inmediatamente con `400 Bad Request` y el error tipado [`crud.ErrBatchTooLarge`](../errors.go) (`"crud: batch size exceeds limit"`).

### Mitigación V2: Bloqueo de Mass Assignment y Columnas Inmutables en `MountUpdate`
- `MountUpdate` valida cada campo en `in.Fields` contra el esquema del modelo (`sample.Schema()`):
  1. **Existencia:** Si el campo no existe en el esquema, rechaza con `400 Bad Request` y [`crud.ErrUnknownField`](../errors.go) (`"crud: unknown field"`).
  2. **Inmutabilidad:** Si el campo es clave primaria (`field.IsPK()` o `"id"`) o la columna de partición multi-inquilino (`"tenant_id"`), rechaza con `400 Bad Request` y [`crud.ErrInvalidField`](../errors.go) (`"crud: immutable field"`).
  3. **No Duplicidad:** Se valida que no existan campos repetidos maliciosamente en la lista.

### Mitigación V3 y V7: Rechazo de Identificadores Vacíos, Duplicados o con Longitud Ilegal
- En `validateIDs`:
  - Se verifica que cada ID no sea una cadena vacía ni contenga solo espacios (`strings.TrimSpace(id) == ""`).
  - Se verifica que ningún ID supere `MaxID = 128` caracteres.
  - Se detectan y rechazan IDs repetidos en el mismo lote con `ErrInvalidID` mediante un recorrido sin alocaciones en heap.

### Mitigación V4 y V6: Validación Declarativa de Esquema en Creación y Actualización Parcial
- En `MountCreate` (`validateCreateBatch`): se valida cada registro del lote usando `model.Validator` / `model.ValidateFields(model.ActionCreate, rec)`. Cualquier violación de caracteres permitidos (`Permitted`), campos requeridos (`NotNull`) o longitudes responde inmediatamente `400 Bad Request`.
- En `MountUpdate` (`validateUpdateFields`): para cada campo listado en `in.Fields`, se extrae su puntero en el registro y se valida contra las reglas declaradas en su `model.Field` (`field.Validate(val)` y `NotNull`), impidiendo inyección de datos ilegales en actualizaciones parciales.

### Mitigación V5 y V8: Acotamiento de Términos de Búsqueda, Cursor y Longitudes (`MaxSearch`, `MaxAfter`, `MaxID`)
- Se fijan las cotas máximas en `read.go`:
  - `MaxSearch = 256`: término de búsqueda.
  - `MaxAfter = 128`: cursor de paginación (`keyset pagination`).
  - `MaxID = 128`: identificador de registro.
- En `ReadArgsModel`, los campos declaran estas restricciones mediante `model.Permitted{Maximum: ...}`.
- En `MountRead`, cualquier término `Search`, `After` o `ID` que supere el límite es acotado de inmediato.

---

## 4. Nuevos Errores de Seguridad Tipados

En [`errors.go`](../errors.go) se formalizan los nuevos errores de validación de entrada (`model.Invalid`, que responden automáticamente `400 Bad Request` vía `rpc.WriteError`):

```go
const (
	ErrNoRecords     model.Invalid = "crud: no records to save"
	ErrNoIDs         model.Invalid = "crud: no ids given"
	ErrNoFields      model.Invalid = "crud: no fields given"
	ErrCannotRead    model.Invalid = "crud: cannot read the request"
	ErrBatchTooLarge model.Invalid = "crud: batch size exceeds limit"
	ErrInvalidID     model.Invalid = "crud: empty id"
	ErrInvalidField  model.Invalid = "crud: immutable field"
	ErrUnknownField  model.Invalid = "crud: unknown field"
)
```

---

## 5. Pruebas Automatizadas de Verificación

El archivo [`tests/security_test.go`](../tests/security_test.go) prueba de forma exhaustiva cada vector de ataque con `loopback`:
- `TestSecurity_BatchLimitExceeded`: verifica el rechazo seguro de lotes masivos en `Create`, `Update` y `Delete` con `> 500` elementos.
- `TestSecurity_ImmutableAndUnknownFields`: verifica el bloqueo de modificación de `"id"`, `"tenant_id"`, columnas desconocidas y duplicadas en `Update`.
- `TestSecurity_InvalidOrEmptyIDs`: verifica el rechazo con `ErrInvalidID` ante IDs vacíos `""` o con solo espacios `"   "`.
- `TestSecurity_CreateSchemaValidation`: verifica que violaciones de esquemas declarativos (`Permitted.Maximum`, `NotNull`, etc.) se detengan en el borde con error 400.
- `TestSecurity_SearchTermCapped`: verifica que consultas con `Search` de más de 256 caracteres se acoten de forma segura a `MaxSearch = 256`.
- `TestSecurity_DuplicateIDs`: verifica el rechazo de identificadores duplicados en peticiones de `Update` y `Delete`.
- `TestSecurity_UpdateValueValidation`: verifica que los valores del registro portador en actualizaciones parciales respeten las restricciones declarativas (`Permitted.Maximum`).
- `TestSecurity_CursorAndIDCapped`: verifica el acotamiento de `After` (`MaxAfter = 128`) e `ID` (`MaxID = 128`).

Ejecución de suite:
```
vet ✅, race ✅, tests ✅, coverage: 83.9%
```

---

## 6. Centralización de Infraestructura en `op.go` y Validaciones en `validate.go`

Para eliminar la duplicación de código entre las 4 operaciones y asegurar que cada archivo de dominio (`create.go`, `read.go`, `update.go`, `delete.go`) contenga únicamente la lógica específica de su verbo:

1. **`mountOp` (`op.go`):** centraliza el registro en el enrutador (`reg.Operation(Op(m, action), ...)`), la resolución canónica del nombre, el enganche de control de acceso (`Requires(res, action)`) y la declaración de esquemas aceptados (`Accepts(sample)`).
2. **`validateIDs` (`validate.go`):** centraliza la validación de lotes de IDs compartida entre `Update` y `Delete` (techo `MaxPage`, detección de vacíos/espacios, límite de longitud `MaxID` y detección de duplicados con cero alocaciones en heap).
3. **`validateCreateBatch` (`validate.go`):** centraliza el chequeo de lote y la validación declarativa de esquema para todos los elementos en `Create`.
4. **`validateUpdateFields` (`validate.go`):** centraliza la validación de campos modificables (inmutabilidad de PK y partición multi-inquilino), descarte de desconocidos, duplicados y validación de los valores actualizados.

---

## 7. Estado y Veredicto

**Estado:** **MITIGADO, REFACTORIZADO Y VERIFICADO (100 %)**.
Con estas 8 defensas activas y la arquitectura centralizada en `webtyp/crud`, el código es más limpio, modular y seguro por defecto.

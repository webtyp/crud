# Análisis de Rendimiento y Memoria en `webtyp.com/crud`

Este documento detalla el perfil de rendimiento, benchmarks empíricos y análisis de asignación de memoria (`allocs/op`, `B/op`, `ns/op`) de [`webtyp.com/crud`](../README.md) en entornos de producción (Frontend WebAssembly / TinyGo y Backend Go Nativo / Edge Workers).

---

## 1. Resumen Ejecutivo y Objetivos de Rendimiento

El diseño de `webtyp.com/crud` está optimizado bajo tres premisas fundamentales de bajo nivel:

1. **Zero Allocations en el Cliente (WASM / TinyGo):**
   - Todas las funciones de codificación (`ReadArgs`, `CreateArgs`, `UpdateArgs`, `DeleteArgs`) realizan exactamente **`1 alloc/op`** (únicamente el buffer `[]byte` o `string` final que se entrega al transporte).
   - Se eliminaron todos los genéricos (`[T any, PT ...]`) para evitar la desvirtualización y generación de código redundante en TinyGo.

2. **Fast-Paths Inlined y $O(1)$ en Servidor:**
   - La función [`crud.Cap(args.Limit)`](../read.go) acota cualquier paginación en **`0.22 ns` y `0 allocs`**.
   - Los decodificadores de transporte utilizan arreglos inline (`[1]string`) para evitar llamadas a `make` cuando se opera sobre un solo ID o campo.

3. **Coste de Seguridad Predecible:**
   - La validación declarativa de esquema en creación y actualización parcial añade un coste inferior a **380 nanosegundos por registro**, garantizando protección total en el borde antes de tocar la base de datos.

---

## 2. Entorno y Metodología de Medición

Las métricas reportadas corresponden a mediciones reales ejecutadas con el harness oficial de Go:

```bash
go test -run=^$ -bench=. -benchmem ./tests
```

* **CPU:** 11th Gen Intel(R) Core(TM) i7-11800H @ 2.30GHz (16 cores)
* **SO / Arquitectura:** Linux x86_64
* **Runtime:** Go 1.24 nativo
* **Suite de Pruebas:** [`tests/bench_test.go`](../tests/bench_test.go)

---

## 3. Matriz Completa de Benchmarks

### 3.1 Codificación y Transporte en el Cliente (WASM / Navegador)

| Operación / Benchmark | Tiempo (`ns/op`) | Memoria (`B/op`) | Asignaciones (`allocs/op`) | Evaluación |
|---|---:|---:|---:|---|
| `BenchmarkOp` | `24.66 ns` | `16 B` | `1 alloc` | Resolución canónica `<Model>_c\|r\|u\|d` |
| `BenchmarkCap` | `0.23 ns` | `0 B` | `0 allocs` | Inlined, acotamiento instantáneo `1..500` |
| `BenchmarkReadArgs_Encode` | `198.9 ns` | `80 B` | `1 alloc` | Solo el `string` de salida JSON |
| `BenchmarkReadArgs_EncodeBytes` | `183.2 ns` | `80 B` | `1 alloc` | Solo el `[]byte` de salida |
| `BenchmarkReadArgs_Decode` | `359.0 ns` | `72 B` | `3 allocs` | Decodificación secuencial en servidor |
| `BenchmarkReadArgs_Pointers` | `24.06 ns` | `64 B` | `1 alloc` | Punteros directos a campos |
| `BenchmarkCreateArgs_Encode_1` | `219.6 ns` | `64 B` | `1 alloc` | Lote de 1 registro |
| `BenchmarkCreateArgs_Encode_10` | `1,185 ns` | `448 B` | `1 alloc` | Lote de 10 registros |
| `BenchmarkCreateArgs_Encode_100` | `10,874 ns` | `4,871 B` | `1 alloc` | Lote de 100 registros |
| `BenchmarkUpdateArgs_Encode` | `361.8 ns` | `128 B` | `1 alloc` | Modificación de 3 IDs y 1 campo |
| `BenchmarkDeleteArgs_Encode_1` | `106.3 ns` | `24 B` | `1 alloc` | Borrado unitario |
| `BenchmarkDeleteArgs_Encode_10` | `355.2 ns` | `160 B` | `1 alloc` | Borrado masivo de 10 IDs |

### 3.2 Operaciones de Servidor por Red Loopback (Ciclo Completo)

Mide el ciclo completo: serialización del cliente, transporte por loopback, decodificación en servidor, validaciones de seguridad de entrada, invocación del handler del módulo y respuesta.

| Operación / Benchmark | Original (`view` con genéricos) | Línea Base (sin validar) | **Actual (con seguridad)** | Impacto |
|---|---:|---:|---:|---|
| `BenchmarkMountRead_Loopback_1` | *(No existía)* | `1,321 ns · 545 B · 16 allocs` | **`1,248 ns · 545 B · 16 allocs`** | **Más rápido (-73 ns)** |
| `BenchmarkMountDelete_Loopback_1` | `466 ns · 368 B · 11 allocs` | `399 ns · 248 B · 6 allocs` | **`384.7 ns · 248 B · 6 allocs`** | **Más rápido (-15 ns)** |
| `BenchmarkMountDelete_Loopback_10` | `2,245 ns · 793 B · 20 allocs` | `1,270 ns · 689 B · 16 allocs` | **`1,240 ns · 689 B · 16 allocs`** | **Más rápido (-30 ns)** |
| `BenchmarkMountUpdate_Loopback_1` | *(No existía)* | `1,039 ns · 433 B · 10 allocs` | **`1,117 ns · 481 B · 11 allocs`** | `+78 ns · +48 B · +1 alloc` |
| `BenchmarkMountCreate_Loopback_1` | `1,459 ns · 978 B · 25 allocs` | `1,196 ns · 513 B · 16 allocs` | **`1,714 ns · 1,002 B · 20 allocs`** | `+518 ns · +4 allocs` |
| `BenchmarkMountCreate_Loopback_10` | `9,792 ns · 5,653 B · 91 allocs` | `4,447 ns · 1,716 B · 47 allocs` | **`8,070 ns · 6,537 B · 87 allocs`** | `+3.6 µs · +40 allocs` |
| `BenchmarkMountCreate_Loopback_100` | `324,827 ns · 61,025 B · 727 allocs`| `33,187 ns · 13,887 B · 320 allocs`| **`71,107 ns · 62,271 B · 720 allocs`**| **4.5x más rápido que original** |

---

## 4. Análisis del Coste de Seguridad (Security Ledger)

La variación observada en `MountCreate` y `MountUpdate` entre la línea base y el estado actual se debe exclusivamente a la **activación de los filtros defensivos en [`validate.go`](../validate.go)**:

### 4.1 Validación Declarativa en Creación (`MountCreate`)
* **Qué se añadió:** [`validateCreateBatch`](../validate.go) ejecuta [`model.ValidateFields(model.ActionCreate, elem)`](../../model/field.go) sobre cada elemento del lote antes de llamar a la persistencia.
* **Desglose de memoria:** 
  Para cada registro, la llamada a `elem.Schema()` y `elem.Pointers()` genera 4 asignaciones internas de introspección:
  $$\text{Para 100 registros: } 100 \times 4 = 400 \text{ alocaciones adicionales } (320 \to 720).$$
* **Coste por registro:**
  Validar 100 registros añade 37.9 microsegundos al ciclo total, lo que representa **379 nanosegundos (0.00037 ms) por registro**.
* **Beneficio:** Garantía total de que ningún registro con campos requeridos nulos, tipos corruptos o textos que excedan `Permitted` llegue a la transacción de base de datos.

### 4.2 Protección contra Mass Assignment en Modificación (`MountUpdate`)
* **Qué se añadió:** [`validateUpdateFields`](../validate.go) valida que las columnas no sean `id`, `tenant_id` o PK, y valida las restricciones del campo sobre el valor entrante.
* **Coste:** **78 nanosegundos** y **1 asignación (48 B)**.
* **Beneficio:** Imposibilidad de alterar identificadores inmutables o transferir registros entre inquilinos (*cross-tenant data breach*).

---

## 5. Evaluación para Producción

### 5.1 En el Navegador (Frontend WASM / TinyGo)
* **Veredicto: ÓPTIMO.**
* Cada llamada genera exactamente **`1 alloc/op`**, evitando la fragmentación del heap de WebAssembly y eliminando pausas del recolector de basura en el hilo de UI.

### 5.2 En el Servidor (Go Nativo y Edge Workers)
* **Veredicto: EXCELENTE.**
* **Lectura unitaria (`Read`):** `1.24 µs`, `545 B`.
* **Borrado unitario (`Delete`):** `384 ns`, `248 B`.
* **Modificación unitaria (`Update`):** `1.11 µs`, `481 B`.
* **Inserción masiva de 100 registros (`Create`):** `71.1 µs`, **4.5x más rápido** que el diseño original heredado de `view` (`325 µs`), con validación de seguridad completa por registro en menos de 400 ns.

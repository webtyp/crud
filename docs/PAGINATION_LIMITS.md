# Límites de Paginación (`MaxPage = 500`, `DefaultPage = 50` y `Cap`)

Este documento explica el origen técnico de la constante [`MaxPage = 500`](../read.go) y la función [`Cap`](../read.go) en `webtyp.com/crud`, cómo interactúan con [`view.DefaultPage`](../../view/page.go) (`50`) y `view.All` (`-1`), y cómo se comparan con los estándares de la industria.

---

## 1. ¿De Dónde Nació la Necesidad?

La paginación acotada por servidor nació en la decisión **U4 (D84)** de [`APP_USABILITY_MASTER_PLAN.md`](../../docs/APP_USABILITY_MASTER_PLAN.md) al importar la base de datos clínica real de **Pa100T** (`mjosefa-cms`):
- **45.973** pacientes
- **171.170** reservas de citas
- **64.356** fichas clínicas

Antes de este cambio, al abrir una vista el servidor ejecutaba un `SELECT` sin `LIMIT` y enviaba toda la tabla al navegador para filtrarla en memoria. Con decenas de miles de registros, una sola consulta serializaba megabytes de JSON y bloqueaba tanto el servidor como el hilo principal del navegador WASM.

---

## 2. El Rol de `Cap` y `MaxPage`

En [`read.go`](../read.go):

```go
const MaxPage = 500

func Cap(limit int64) int64 {
	if limit <= 0 || limit > MaxPage {
		return MaxPage
	}
	return limit
}
```

Cumple dos funciones arquitectónicas:

1. **Protección del servidor contra consultas ilimitadas (`Cap`):**
   Ningún cliente (ni una petición manipulada enviando `{"limit": 0}`, `{"limit": -1}` o `{"limit": 1000000}`) puede forzar al servidor a volcar una tabla entera de SQLite/PostgreSQL en memoria. [`MountRead`](../mount.go) aplica `args.Limit = Cap(args.Limit)` automáticamente en `0.22 ns` y `0 allocs/op`, garantizando que toda consulta `Read` devuelva siempre entre `1` y `500` filas por llamada.

2. **Techo para catálogos completos (`view.All`):**
   En `webtyp.com/view`, las vistas se dividen en dos categorías:
   - **Tablas que crecen sin límite** (pacientes, pagos, auditoría, mensajes): usan `DefaultPage` (`50` filas) o páginas chicas (`Size: 20`, `More: true`) y cargan más registros por cursor (`After`) al hacer scroll o buscar en el servidor.
   - **Catálogos acotados de administración y selectores `<select>`** (pisos, salas, personal, roles, catálogo de prestaciones, equipos): declaran `view.Page{First: view.All}` porque el administrador o el formulario necesita todas las opciones disponibles al abrir la pantalla. Cuando el cliente solicita `view.All`, envía `Limit: crud.MaxPage` (`500`).

---

## 3. Comparativa con los Estándares de la Industria

En la industria existen dos escalas estándar según el propósito de la API: **`100`** para APIs públicas paginadas por cursor y **`500` a `1000`** para plataformas Cloud, ERPs y sistemas de gestión interna.

| Plataforma / Estándar de la Industria | Tamaño por defecto (`DefaultPage`) | Tope máximo por llamada (`MaxPage`) |
|---|---:|---:|
| **GitHub API** (REST y GraphQL) | `30` | **`100`** |
| **Stripe API** | `10` | **`100`** |
| **Linear / Notion / Slack API** | `50` | **`100` – `200`** |
| **Shopify Admin API** | `50` | **`250`** |
| **Google Cloud (AIP-158) / Kubernetes API** | `50` – `100` | **`500`** |
| **PocketBase / Directus (BaaS en Go/Node)** | `30` – `50` | **`500`** |
| **Supabase / PostgREST** | `100` | **`1000`** |
| **`webtyp` (`crud` + `view`)** | **`50`** (`20` en búsqueda de pacientes) | **`500`** |

### ¿Por qué `webtyp` combina `DefaultPage = 50` con `MaxPage = 500`?

- **En listados paginados (`DefaultPage = 50`, o `20` en pacientes):** `webtyp` opera en el mismo rango estándar de la industria (**`20` a `50` filas por página**).
- **En el techo máximo (`MaxPage = 500` en lugar de `100`):**
  Como `view.All` se utiliza también para poblar catálogos administrativos y selectores (`<select>`), un tope de `100` truncaría catálogos habituales en producción (por ejemplo, un arancel clínico de **150–300 prestaciones médicas** o las **346 comunas de Chile**). Con **`500`** (alineado con Google AIP-158, Kubernetes y PocketBase):
  - Cualquier catálogo o selector de dominio entra completo en una sola petición (`< 50 KB` en JSON).
  - El servidor y el árbol DOM del navegador quedan protegidos frente a tablas masivas de decenas de miles de filas.

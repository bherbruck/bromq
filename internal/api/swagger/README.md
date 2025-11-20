# OpenAPI/Swagger Documentation

This directory contains auto-generated OpenAPI/Swagger documentation for the BroMQ REST API.

## Files

- **`embed.go`** - Simple go:embed wrapper for swagger files (289 bytes)
- **`swagger.json`** - OpenAPI 2.0 specification (JSON) - regenerated on build (gitignored)
- **`swagger.yaml`** - OpenAPI 2.0 specification (YAML) - regenerated on build (gitignored)
- **`README.md`** - This documentation

## Accessing the Documentation

After starting the server, access the interactive Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

The raw OpenAPI spec is available at:

```
http://localhost:8080/swagger/doc.json
```

## Testing Authenticated Endpoints

The Swagger UI is publicly accessible, but most API endpoints require authentication.

### Authentication Flow:

1. **Get a JWT token** - Click "Try it out" on the `POST /auth/login` endpoint:
   ```json
   {
     "username": "admin",
     "password": "admin"
   }
   ```

2. **Copy the token** from the response `token` field

3. **Authorize** - Click the "Authorize" button (green lock icon, top right)

4. **Enter token** - Paste: `Bearer <your-token>`

5. **Test endpoints** - All subsequent requests will include the JWT token

### Default Credentials

On first run, BroMQ creates a default admin user:
- **Username:** `admin` (override with `ADMIN_USERNAME`)
- **Password:** `admin` (override with `ADMIN_PASSWORD`)

Change these immediately in production!

## Regenerating Documentation

Documentation is automatically regenerated when you build the project:

```bash
make build
```

To regenerate documentation manually:

```bash
make swagger
```

## Adding API Documentation

The main API metadata (title, version, description, contact, etc.) is in `internal/api/doc.go`.

To document a new endpoint, add swagger annotations above the handler function in the appropriate handler file:

```go
// CreateUser godoc
// @Summary Create a new user
// @Description Create a new user with the provided details
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param user body CreateUserRequest true "User details"
// @Success 201 {object} User
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /users [post]
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // implementation
}
```

### Common Annotations

- `@Summary` - Short description (appears in list)
- `@Description` - Longer description
- `@Tags` - Groups endpoints together
- `@Accept` - Content-Type the endpoint accepts (usually `json`)
- `@Produce` - Content-Type the endpoint produces (usually `json`)
- `@Security BearerAuth` - Requires JWT authentication
- `@Param` - Request parameters (path, query, body, header)
- `@Success` - Success response with status code and type
- `@Failure` - Error response with status code and type
- `@Router` - API path and HTTP method

### Parameter Types

- **path** - URL path parameter (e.g., `/users/{id}`)
- **query** - Query string parameter (e.g., `?page=1`)
- **body** - Request body (JSON)
- **header** - HTTP header

## Examples

See existing handlers in `../` for comprehensive examples:
- `dashboard_handlers.go` - Dashboard user management
- `mqtt_handlers.go` - MQTT users and clients
- `bridge_handlers.go` - MQTT bridges
- `script_handlers.go` - JavaScript scripts
- `handlers.go` - ACL, clients, metrics

## Resources

- [Swaggo Documentation](https://github.com/swaggo/swag)
- [Declarative Comments Format](https://github.com/swaggo/swag#declarative-comments-format)
- [OpenAPI Specification](https://swagger.io/specification/)

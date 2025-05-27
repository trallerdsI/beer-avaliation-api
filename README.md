# Beer Review Application

A modern, scalable REST API for managing beer reviews and ratings built with Go.

## Features

- 🍺 Comprehensive beer catalog management
- 💬 User comments and ratings
- 🔍 Advanced search with filters
- 📊 Monitoring and metrics
- 🔐 Authentication and authorization
- 🔄 Circuit breaker pattern
- 📝 Swagger documentation

## Architecture

The application follows clean architecture principles with the following layers:

```
beer-review-app/
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── beer/           # Beer domain
│   │   ├── delivery/   # HTTP handlers
│   │   ├── repository/ # Data access layer
│   │   ├── usecase/    # Business logic
│   │   └── model/      # Domain models
│   ├── user/           # User domain
│   └── monitoring/     # Monitoring components
├── pkg/
│   ├── middleware/     # HTTP middleware
│   ├── errors/        # Error handling
│   └── response/      # HTTP response helpers
```

## Prerequisites

- Go 1.23+
- PostgreSQL 14+
- Docker (optional)

## Getting Started

1. Clone the repository:
```bash
git clone https://github.com/yourusername/beer-review-app.git
cd beer-review-app
```

2. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. Run the application:
```bash
go run cmd/server/main.go
```

## API Endpoints

### Beer Operations
- `GET /api/v1/beers` - List all beers
- `POST /api/v1/beers` - Create a new beer
- `GET /api/v1/beers/{id}` - Get beer details
- `PUT /api/v1/beers/{id}` - Update beer
- `DELETE /api/v1/beers/{id}` - Delete beer
- `GET /api/v1/beers/search` - Search beers with filters

### Comments
- `POST /api/v1/beers/{id}/comments` - Add comment
- `DELETE /api/v1/beers/{id}/comments/{commentId}` - Delete comment
- `POST /api/v1/beers/{id}/comments/{commentId}/like` - Like comment

### User Operations
- `POST /api/v1/users/register` - Register new user
- `POST /api/v1/users/login` - User login
- `GET /api/v1/users/{id}` - Get user profile
- `PUT /api/v1/users/{id}` - Update profile
- `DELETE /api/v1/users/{id}` - Delete account

### Monitoring
- `GET /api/v1/health` - Health check
- `GET /api/v1/stats` - Application statistics
- `GET /metrics` - Prometheus metrics

## Testing

Run the test suite:
```bash
go test ./...
```

Run with coverage:
```bash
go test -cover ./...
```

## Deployment

The application can be deployed using Docker:

```bash
docker build -t beer-review-app .
docker run -p 8080:8080 beer-review-app
```

## Monitoring

The application exposes metrics for Prometheus at `/metrics` and includes:
- Response times
- Error counts
- Request counts

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
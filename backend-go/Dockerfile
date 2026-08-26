# syntax=docker/dockerfile:1
# Build stage -----------------------------------------------------------------
FROM golang:1.26.4-bookworm AS build
WORKDIR /src

# Copy the module files first so the dependency download is cached independently
# of source edits (same idea as the restore layer in backend/Dockerfile).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off + distroless static means no libc to link against at run time.
# -trimpath keeps build paths out of the binary, -s -w drops the symbol table.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage ---------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

# Render overrides PORT at run time; 8081 is the local default and keeps the Go
# API off the .NET API's port during parity runs. There is no SQLite fallback
# here: DATABASE_URL is required and the schema comes from db/migrations.
ENV PORT=8081 \
    GIN_MODE=release

COPY --from=build /out/api ./api

# The :nonroot tag already defaults to uid 65532; stated so it survives edits.
USER nonroot:nonroot

EXPOSE 8081
ENTRYPOINT ["/app/api"]

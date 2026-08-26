# syntax=docker/dockerfile:1
# Build stage -----------------------------------------------------------------
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
WORKDIR /src

COPY FinanceTracker.sln ./
COPY FinanceTracker.Api/FinanceTracker.Api.csproj FinanceTracker.Api/
COPY FinanceTracker.Tests/FinanceTracker.Tests.csproj FinanceTracker.Tests/
RUN dotnet restore FinanceTracker.Api/FinanceTracker.Api.csproj

COPY . .
RUN dotnet publish FinanceTracker.Api/FinanceTracker.Api.csproj \
    -c Release \
    -o /app/publish \
    --no-restore \
    /p:UseAppHost=false

# Runtime stage ---------------------------------------------------------------
FROM mcr.microsoft.com/dotnet/aspnet:10.0 AS runtime
WORKDIR /app

# Render overrides PORT at run time; 8080 is the local default.
# The SQLite fallback lives in a writable volume; production sets DATABASE_URL and uses Postgres.
ENV PORT=8080 \
    DOTNET_RUNNING_IN_CONTAINER=true \
    ASPNETCORE_ENVIRONMENT=Production \
    ConnectionStrings__Sqlite="Data Source=/data/finance.db"

COPY --from=build /app/publish ./

RUN mkdir -p /data && chown -R $APP_UID:$APP_UID /data
VOLUME ["/data"]

# Run as the non-root user shipped with the aspnet image.
USER $APP_UID

EXPOSE 8080
ENTRYPOINT ["dotnet", "FinanceTracker.Api.dll"]

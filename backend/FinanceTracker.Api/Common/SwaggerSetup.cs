using Microsoft.OpenApi;
using Swashbuckle.AspNetCore.SwaggerGen;

namespace FinanceTracker.Api.Common;

/// <summary>Swagger document plus the bearer-token input box.</summary>
public static class SwaggerSetup
{
    private const string SchemeId = "Bearer";

    public static void Configure(SwaggerGenOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);

        options.SwaggerDoc("v1", new OpenApiInfo { Title = "Finance Tracker API", Version = "v1" });

        options.AddSecurityDefinition(SchemeId, new OpenApiSecurityScheme
        {
            Name = "Authorization",
            Type = SecuritySchemeType.Http,
            Scheme = "bearer",
            BearerFormat = "JWT",
            In = ParameterLocation.Header,
            Description = "Paste the JWT returned by /api/auth/login.",
        });

        options.AddSecurityRequirement(_ => new OpenApiSecurityRequirement
        {
            [new OpenApiSecuritySchemeReference(SchemeId)] = [],
        });
    }
}

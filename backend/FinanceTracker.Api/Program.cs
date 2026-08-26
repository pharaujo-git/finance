using System.Text.Json;
using System.Text.Json.Serialization;
using FinanceTracker.Api.BackgroundJobs;
using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Models;
using FinanceTracker.Api.Services;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Cors.Infrastructure;
using Microsoft.AspNetCore.Identity;
using Microsoft.IdentityModel.Tokens;
using Microsoft.OpenApi;

var builder = WebApplication.CreateBuilder(args);

// Render (and most PaaS hosts) inject the listening port through PORT.
var port = builder.Configuration["PORT"] ?? "8080";
if (builder.Environment.IsProduction() || Environment.GetEnvironmentVariable("PORT") is not null)
{
    builder.WebHost.UseUrls($"http://0.0.0.0:{port}");
}

// Everything below resolves configuration from DI so that values supplied late
// (environment variables, test overrides) are honoured rather than snapshotted here.
builder.Services.AddSingleton(sp => JwtOptions.FromConfiguration(sp.GetRequiredService<IConfiguration>()));
builder.Services.AddAppDatabase();
builder.Services.AddSingleton<IPasswordHasher<User>, PasswordHasher<User>>();
builder.Services.AddSingleton<ITokenService, TokenService>();
builder.Services.AddScoped<AuthService>();
builder.Services.AddScoped<AccountService>();
builder.Services.AddScoped<CategoryService>();
builder.Services.AddScoped<TransactionService>();
builder.Services.AddScoped<TransactionCsvService>();
builder.Services.AddScoped<RecurringService>();
builder.Services.AddScoped<BudgetService>();
builder.Services.AddScoped<GoalService>();
builder.Services.AddScoped<AnalyticsService>();
builder.Services.AddHostedService<RecurringTransactionWorker>();

builder.Services.AddProblemDetails();
builder.Services.AddExceptionHandler<ApiExceptionHandler>();

builder.Services
    .AddControllers()
    .AddJsonOptions(options =>
    {
        options.JsonSerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.CamelCase;
        options.JsonSerializerOptions.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.CamelCase));
    });

builder.Services.AddCors();
builder.Services.AddOptions<CorsOptions>().Configure<IConfiguration>((options, configuration) =>
    options.AddDefaultPolicy(policy => policy
        .WithOrigins(CorsOrigins.Read(configuration))
        .AllowAnyHeader()
        .AllowAnyMethod()));

builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme).AddJwtBearer();
builder.Services
    .AddOptions<JwtBearerOptions>(JwtBearerDefaults.AuthenticationScheme)
    .Configure<JwtOptions>((options, jwt) => options.TokenValidationParameters = new TokenValidationParameters
    {
        ValidateIssuer = true,
        ValidateAudience = true,
        ValidateLifetime = true,
        ValidateIssuerSigningKey = true,
        ValidIssuer = jwt.Issuer,
        ValidAudience = jwt.Audience,
        IssuerSigningKey = jwt.SigningKey(),
        ClockSkew = TimeSpan.FromMinutes(1),
    });

builder.Services.AddAuthorization();
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen(SwaggerSetup.Configure);

var app = builder.Build();

await using (var scope = app.Services.CreateAsyncScope())
{
    var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
    await db.Database.EnsureCreatedAsync();
    await DefaultCategorySeeder.SeedAsync(db);
}

if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseExceptionHandler();
app.UseCors();
app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();
app.MapGet("/health", () => Results.Text("ok")).AllowAnonymous();

await app.RunAsync();

/// <summary>Reads the comma-separated <c>ALLOWED_ORIGINS</c> variable.</summary>
internal static class CorsOrigins
{
    public const string Variable = "ALLOWED_ORIGINS";

    public const string Default = "http://localhost:5173";

    public static string[] Read(IConfiguration configuration)
    {
        var raw = configuration[Variable];
        var value = string.IsNullOrWhiteSpace(raw) ? Default : raw;
        return value.Split(',', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);
    }
}

/// <summary>Swagger document plus the bearer-token input box.</summary>
internal static class SwaggerSetup
{
    public static void Configure(Swashbuckle.AspNetCore.SwaggerGen.SwaggerGenOptions options)
    {
        options.SwaggerDoc("v1", new OpenApiInfo { Title = "Finance Tracker API", Version = "v1" });

        options.AddSecurityDefinition("Bearer", new OpenApiSecurityScheme
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
            [new OpenApiSecuritySchemeReference("Bearer")] = [],
        });
    }
}

/// <summary>Exposed so the integration tests can boot the real pipeline.</summary>
public partial class Program;

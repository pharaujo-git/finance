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

var builder = WebApplication.CreateBuilder(args);

// Render (and most PaaS hosts) inject the listening port through PORT and terminate TLS
// at their edge proxy, so Kestrel binds a plain socket on every interface inside the container.
if (builder.Environment.IsProduction() || builder.Configuration[ServerBinding.PortVariable] is not null)
{
    var port = ServerBinding.ResolvePort(builder.Configuration);
    builder.WebHost.ConfigureKestrel(options => options.ListenAnyIP(port));
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

/// <summary>
/// The entry-point class generated from the top-level statements above. It stays in the global
/// namespace because that is where the compiler emits it, and it is made public so the
/// integration tests can boot the real pipeline through <c>WebApplicationFactory</c>.
/// </summary>
public partial class Program
{
    /// <summary>The runtime invokes the generated entry point; nothing constructs this type.</summary>
    protected Program()
    {
    }
}

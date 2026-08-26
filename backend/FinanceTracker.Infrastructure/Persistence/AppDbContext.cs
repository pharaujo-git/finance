using FinanceTracker.Application.Abstractions;
using FinanceTracker.Domain;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Storage.ValueConversion;

namespace FinanceTracker.Infrastructure.Persistence;

/// <summary>EF Core context for every entity in the app.</summary>
public sealed class AppDbContext(DbContextOptions<AppDbContext> options) : DbContext(options), IAppDbContext
{
    /// <summary>SQLite forgets <see cref="DateTimeKind"/>; this restores it so the API always emits UTC.</summary>
    private static readonly ValueConverter<DateTime, DateTime> UtcConverter =
        new(value => value, value => DateTime.SpecifyKind(value, DateTimeKind.Utc));

    private static readonly ValueConverter<DateTime?, DateTime?> NullableUtcConverter =
        new(value => value, value => value.HasValue ? DateTime.SpecifyKind(value.Value, DateTimeKind.Utc) : null);

    public DbSet<User> Users => Set<User>();

    public DbSet<Account> Accounts => Set<Account>();

    public DbSet<Category> Categories => Set<Category>();

    public DbSet<Transaction> Transactions => Set<Transaction>();

    public DbSet<RecurringRule> RecurringRules => Set<RecurringRule>();

    public DbSet<Budget> Budgets => Set<Budget>();

    public DbSet<Goal> Goals => Set<Goal>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        ArgumentNullException.ThrowIfNull(modelBuilder);
        base.OnModelCreating(modelBuilder);

        modelBuilder.Entity<User>(entity =>
        {
            entity.HasIndex(e => e.Email).IsUnique();
            entity.Property(e => e.Email).HasMaxLength(256).IsRequired();
            entity.Property(e => e.Name).HasMaxLength(200).IsRequired();
            entity.Property(e => e.PasswordHash).HasMaxLength(512).IsRequired();
            entity.Property(e => e.Currency).HasMaxLength(8).IsRequired();
        });

        modelBuilder.Entity<Account>(entity =>
        {
            entity.HasIndex(e => e.UserId);
            entity.Property(e => e.Name).HasMaxLength(200).IsRequired();
            entity.Property(e => e.Currency).HasMaxLength(8).IsRequired();
            entity.Property(e => e.InitialBalance).HasPrecision(18, 2);
        });

        modelBuilder.Entity<Category>(entity =>
        {
            entity.HasIndex(e => e.UserId);
            entity.Property(e => e.Name).HasMaxLength(200).IsRequired();
            entity.Property(e => e.Icon).HasMaxLength(64);
            entity.Property(e => e.Color).HasMaxLength(32);
        });

        modelBuilder.Entity<Transaction>(entity =>
        {
            entity.HasIndex(e => new { e.UserId, e.Date });
            entity.HasIndex(e => e.AccountId);
            entity.Property(e => e.Description).HasMaxLength(500).IsRequired();
            entity.Property(e => e.Notes).HasMaxLength(2000);
            entity.Property(e => e.TagsRaw).HasMaxLength(1000).IsRequired();
            entity.Property(e => e.Amount).HasPrecision(18, 2);
        });

        modelBuilder.Entity<RecurringRule>(entity =>
        {
            entity.HasIndex(e => new { e.UserId, e.NextRunDate });
            entity.Property(e => e.Description).HasMaxLength(500).IsRequired();
            entity.Property(e => e.Amount).HasPrecision(18, 2);
        });

        modelBuilder.Entity<Budget>(entity =>
        {
            entity.HasIndex(e => new { e.UserId, e.Month });
            entity.Property(e => e.Month).HasMaxLength(7).IsRequired();
            entity.Property(e => e.Limit).HasPrecision(18, 2);
        });

        modelBuilder.Entity<Goal>(entity =>
        {
            entity.HasIndex(e => e.UserId);
            entity.Property(e => e.Name).HasMaxLength(200).IsRequired();
            entity.Property(e => e.Color).HasMaxLength(32);
            entity.Property(e => e.TargetAmount).HasPrecision(18, 2);
            entity.Property(e => e.CurrentAmount).HasPrecision(18, 2);
        });

        foreach (var property in modelBuilder.Model.GetEntityTypes().SelectMany(e => e.GetProperties()))
        {
            if (property.ClrType == typeof(DateTime))
            {
                property.SetValueConverter(UtcConverter);
            }
            else if (property.ClrType == typeof(DateTime?))
            {
                property.SetValueConverter(NullableUtcConverter);
            }
        }
    }
}

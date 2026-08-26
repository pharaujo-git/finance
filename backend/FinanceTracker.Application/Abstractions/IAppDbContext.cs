using FinanceTracker.Domain;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Application.Abstractions;

/// <summary>
/// The persistence surface the application services need. Implemented by the EF Core context in
/// Infrastructure, which keeps provider choice (SQLite vs Npgsql) out of this layer.
/// </summary>
public interface IAppDbContext
{
    DbSet<User> Users { get; }

    DbSet<Account> Accounts { get; }

    DbSet<Category> Categories { get; }

    DbSet<Transaction> Transactions { get; }

    DbSet<RecurringRule> RecurringRules { get; }

    DbSet<Budget> Budgets { get; }

    DbSet<Goal> Goals { get; }

    Task<int> SaveChangesAsync(CancellationToken cancellationToken = default);
}

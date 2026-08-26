namespace FinanceTracker.Domain;

/// <summary>Entity that belongs to exactly one user; enables a single generic ownership check.</summary>
public interface IUserOwned
{
    Guid Id { get; }

    Guid UserId { get; }
}

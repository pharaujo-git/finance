using FinanceTracker.Api.Common;
using FinanceTracker.Api.Data;
using FinanceTracker.Api.Dtos;
using FinanceTracker.Api.Models;
using Microsoft.EntityFrameworkCore;

namespace FinanceTracker.Api.Services;

/// <summary>Recurring rule CRUD plus the materialization pass run by the background worker.</summary>
public sealed class RecurringService(AppDbContext db, CategoryService categories)
{
    private const string EntityName = "Recurring rule";

    /// <summary>Upper bound on catch-up occurrences per rule per pass, so a stale rule cannot loop forever.</summary>
    private const int MaxOccurrencesPerPass = 500;

    public static DateTime Advance(DateTime from, Frequency frequency) => frequency switch
    {
        Frequency.Daily => from.AddDays(1),
        Frequency.Weekly => from.AddDays(7),
        Frequency.Monthly => from.AddMonths(1),
        Frequency.Yearly => from.AddYears(1),
        _ => from.AddMonths(1),
    };

    public async Task<IReadOnlyList<RecurringRuleDto>> ListAsync(Guid userId, CancellationToken cancellationToken)
    {
        var rules = await db.RecurringRules
            .AsNoTracking()
            .OwnedBy(userId)
            .OrderBy(r => r.NextRunDate)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        return rules.Select(Map).ToList();
    }

    public async Task<RecurringRuleDto> CreateAsync(
        Guid userId,
        RecurringRuleRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        await ValidateAsync(userId, request, cancellationToken).ConfigureAwait(false);

        var rule = new RecurringRule { UserId = userId };
        Apply(rule, request);
        rule.NextRunDate = rule.StartDate;

        db.RecurringRules.Add(rule);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);

        return Map(rule);
    }

    public async Task<RecurringRuleDto> UpdateAsync(
        Guid userId,
        Guid id,
        RecurringRuleRequest request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        await ValidateAsync(userId, request, cancellationToken).ConfigureAwait(false);

        var rule = await db.RecurringRules
            .GetOwnedAsync(id, userId, EntityName, cancellationToken)
            .ConfigureAwait(false);

        Apply(rule, request);
        if (rule.NextRunDate < rule.StartDate)
        {
            rule.NextRunDate = rule.StartDate;
        }

        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
        return Map(rule);
    }

    public async Task DeleteAsync(Guid userId, Guid id, CancellationToken cancellationToken)
    {
        var rule = await db.RecurringRules
            .GetOwnedAsync(id, userId, EntityName, cancellationToken)
            .ConfigureAwait(false);

        db.RecurringRules.Remove(rule);
        await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
    }

    /// <summary>Creates transactions for every occurrence due on or before <paramref name="now"/>.</summary>
    /// <returns>The number of transactions created.</returns>
    public async Task<int> MaterializeDueAsync(DateTime now, CancellationToken cancellationToken)
    {
        var cutoff = UtcDate.Normalize(now);
        var due = await db.RecurringRules
            .Where(r => r.IsActive && r.NextRunDate <= cutoff)
            .ToListAsync(cancellationToken)
            .ConfigureAwait(false);

        var created = new List<Transaction>();
        foreach (var rule in due)
        {
            created.AddRange(MaterializeRule(rule, cutoff));
        }

        if (created.Count > 0)
        {
            db.Transactions.AddRange(created);
        }

        if (created.Count > 0 || due.Count > 0)
        {
            await db.SaveChangesAsync(cancellationToken).ConfigureAwait(false);
        }

        return created.Count;
    }

    private static IEnumerable<Transaction> MaterializeRule(RecurringRule rule, DateTime cutoff)
    {
        for (var occurrence = 0; occurrence < MaxOccurrencesPerPass; occurrence++)
        {
            if (!rule.IsActive || rule.NextRunDate > cutoff)
            {
                yield break;
            }

            if (rule.EndDate is { } end && rule.NextRunDate > end)
            {
                rule.IsActive = false;
                yield break;
            }

            yield return new Transaction
            {
                UserId = rule.UserId,
                AccountId = rule.AccountId,
                CategoryId = rule.CategoryId,
                Type = rule.Type,
                Amount = rule.Amount,
                Date = rule.NextRunDate,
                Description = rule.Description,
                TagsRaw = Transaction.JoinTags(["recurring"]),
                TransferAccountId = null,
            };

            rule.NextRunDate = Advance(rule.NextRunDate, rule.Frequency);
        }
    }

    private static void Apply(RecurringRule rule, RecurringRuleRequest request)
    {
        rule.AccountId = request.AccountId;
        rule.CategoryId = request.CategoryId;
        rule.Type = request.Type;
        rule.Amount = decimal.Round(request.Amount, 2, MidpointRounding.AwayFromZero);
        rule.Description = request.Description.Trim();
        rule.Frequency = request.Frequency;
        rule.StartDate = UtcDate.Normalize(request.StartDate);
        rule.EndDate = UtcDate.Normalize(request.EndDate);
        rule.IsActive = request.IsActive ?? true;
    }

    private async Task ValidateAsync(Guid userId, RecurringRuleRequest request, CancellationToken cancellationToken)
    {
        if (request.Type == TransactionType.Transfer)
        {
            throw ApiException.BadRequest("Recurring transfers are not supported.");
        }

        var accountExists = await db.Accounts
            .AsNoTracking()
            .AnyAsync(a => a.Id == request.AccountId && a.UserId == userId, cancellationToken)
            .ConfigureAwait(false);

        if (!accountExists)
        {
            throw ApiException.NotFound("Account");
        }

        if (request.EndDate is { } end && end < request.StartDate)
        {
            throw ApiException.BadRequest("End date must not be before the start date.");
        }

        await categories.EnsureUsableAsync(userId, request.CategoryId, cancellationToken).ConfigureAwait(false);
    }

    private static RecurringRuleDto Map(RecurringRule rule) => new(
        rule.Id,
        rule.AccountId,
        rule.CategoryId,
        rule.Type,
        rule.Amount,
        rule.Description,
        rule.Frequency,
        rule.StartDate,
        rule.EndDate,
        rule.NextRunDate,
        rule.IsActive);
}

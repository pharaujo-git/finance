using FinanceTracker.Domain;

namespace FinanceTracker.Application.Dtos;

public sealed record DashboardSummaryDto(
    decimal NetWorth,
    decimal TotalIncome,
    decimal TotalExpenses,
    decimal SavingsRate);

public sealed record NetWorthPointDto(string Month, decimal Value);

public sealed record CashflowPointDto(string Month, decimal Income, decimal Expenses);

public sealed record SpendingSliceDto(Guid? CategoryId, string CategoryName, string Color, decimal Amount);

public sealed record MonthlyReportDto(string Month, decimal Income, decimal Expenses, decimal Net);

public sealed record CategoryReportDto(
    Guid? CategoryId,
    string CategoryName,
    CategoryType Type,
    string Color,
    decimal Amount);

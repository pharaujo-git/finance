namespace FinanceTracker.Domain;

/// <summary>Kind of financial account. Serialized as camelCase strings.</summary>
public enum AccountType
{
    Checking,
    Savings,
    CreditCard,
    Cash,
    Investment,
}

/// <summary>Whether a category groups money coming in or going out.</summary>
public enum CategoryType
{
    Income,
    Expense,
}

/// <summary>Direction of a transaction.</summary>
public enum TransactionType
{
    Income,
    Expense,
    Transfer,
}

/// <summary>How often a recurring rule materializes a transaction.</summary>
public enum Frequency
{
    Daily,
    Weekly,
    Monthly,
    Yearly,
}

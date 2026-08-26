-- migrate:up

-- The 18 globally shared default categories that the API used to upsert on every
-- startup. They belong to no user ("UserId" IS NULL) and "Type" is the ordinal of
-- FinanceTracker.Domain.CategoryType (0 = Income, 1 = Expense).
-- Every statement is guarded by NOT EXISTS so this migration is safe to replay and
-- is a no-op against a database the old seeder already populated.
-- gen_random_uuid() is built into PostgreSQL 13+; no pgcrypto extension needed.

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Salary', 0, 'wallet', '#16a34a', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Salary' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Freelance', 0, 'briefcase', '#22c55e', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Freelance' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Investments', 0, 'trending-up', '#0ea5e9', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Investments' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Gifts', 0, 'gift', '#a855f7', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Gifts' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Other Income', 0, 'plus-circle', '#64748b', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Other Income' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Groceries', 1, 'shopping-cart', '#f97316', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Groceries' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Rent', 1, 'home', '#ef4444', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Rent' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Utilities', 1, 'zap', '#eab308', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Utilities' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Transport', 1, 'car', '#3b82f6', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Transport' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Dining', 1, 'utensils', '#f43f5e', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Dining' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Entertainment', 1, 'film', '#8b5cf6', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Entertainment' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Health', 1, 'heart-pulse', '#14b8a6', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Health' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Shopping', 1, 'shopping-bag', '#ec4899', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Shopping' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Education', 1, 'graduation-cap', '#6366f1', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Education' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Travel', 1, 'plane', '#06b6d4', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Travel' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Insurance', 1, 'shield', '#0f766e', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Insurance' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Subscriptions', 1, 'repeat', '#7c3aed', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Subscriptions' AND "IsDefault" = true AND "UserId" IS NULL
);

INSERT INTO "Categories" ("Id", "UserId", "Name", "Type", "Icon", "Color", "IsDefault")
SELECT gen_random_uuid(), NULL, 'Other Expense', 1, 'minus-circle', '#64748b', true
WHERE NOT EXISTS (
    SELECT 1 FROM "Categories"
    WHERE "Name" = 'Other Expense' AND "IsDefault" = true AND "UserId" IS NULL
);

-- migrate:down

DELETE FROM "Categories"
WHERE "IsDefault" = true
  AND "UserId" IS NULL
  AND "Name" IN (
    'Salary',
    'Freelance',
    'Investments',
    'Gifts',
    'Other Income',
    'Groceries',
    'Rent',
    'Utilities',
    'Transport',
    'Dining',
    'Entertainment',
    'Health',
    'Shopping',
    'Education',
    'Travel',
    'Insurance',
    'Subscriptions',
    'Other Expense'
);

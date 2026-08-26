-- migrate:up

-- Baseline schema, captured verbatim from what EF Core's EnsureCreated() used to
-- produce on Postgres. Object and index names keep EF's conventions (PK_*, IX_*,
-- quoted PascalCase identifiers) so an existing database can be baselined into
-- dbmate without any rename. There are no foreign keys: the previous EF model
-- declared none, and this file only records the shape that already shipped.

CREATE TABLE "Users" (
    "Id" uuid NOT NULL,
    "Email" character varying(256) NOT NULL,
    "Name" character varying(200) NOT NULL,
    "PasswordHash" character varying(512) NOT NULL,
    "Currency" character varying(8) NOT NULL,
    "CreatedAt" timestamp with time zone NOT NULL,
    CONSTRAINT "PK_Users" PRIMARY KEY ("Id")
);

CREATE TABLE "Accounts" (
    "Id" uuid NOT NULL,
    "UserId" uuid NOT NULL,
    "Name" character varying(200) NOT NULL,
    "Type" integer NOT NULL,
    "InitialBalance" numeric(18,2) NOT NULL,
    "Currency" character varying(8) NOT NULL,
    "IsArchived" boolean NOT NULL,
    "CreatedAt" timestamp with time zone NOT NULL,
    CONSTRAINT "PK_Accounts" PRIMARY KEY ("Id")
);

CREATE TABLE "Categories" (
    "Id" uuid NOT NULL,
    "UserId" uuid,
    "Name" character varying(200) NOT NULL,
    "Type" integer NOT NULL,
    "Icon" character varying(64) NOT NULL,
    "Color" character varying(32) NOT NULL,
    "IsDefault" boolean NOT NULL,
    CONSTRAINT "PK_Categories" PRIMARY KEY ("Id")
);

CREATE TABLE "Transactions" (
    "Id" uuid NOT NULL,
    "UserId" uuid NOT NULL,
    "AccountId" uuid NOT NULL,
    "CategoryId" uuid,
    "Type" integer NOT NULL,
    "Amount" numeric(18,2) NOT NULL,
    "Date" timestamp with time zone NOT NULL,
    "Description" character varying(500) NOT NULL,
    "Notes" character varying(2000),
    "TagsRaw" character varying(1000) NOT NULL,
    "TransferAccountId" uuid,
    CONSTRAINT "PK_Transactions" PRIMARY KEY ("Id")
);

CREATE TABLE "RecurringRules" (
    "Id" uuid NOT NULL,
    "UserId" uuid NOT NULL,
    "AccountId" uuid NOT NULL,
    "CategoryId" uuid,
    "Type" integer NOT NULL,
    "Amount" numeric(18,2) NOT NULL,
    "Description" character varying(500) NOT NULL,
    "Frequency" integer NOT NULL,
    "StartDate" timestamp with time zone NOT NULL,
    "EndDate" timestamp with time zone,
    "NextRunDate" timestamp with time zone NOT NULL,
    "IsActive" boolean NOT NULL,
    CONSTRAINT "PK_RecurringRules" PRIMARY KEY ("Id")
);

CREATE TABLE "Budgets" (
    "Id" uuid NOT NULL,
    "UserId" uuid NOT NULL,
    "CategoryId" uuid NOT NULL,
    "Month" character varying(7) NOT NULL,
    "Limit" numeric(18,2) NOT NULL,
    CONSTRAINT "PK_Budgets" PRIMARY KEY ("Id")
);

CREATE TABLE "Goals" (
    "Id" uuid NOT NULL,
    "UserId" uuid NOT NULL,
    "Name" character varying(200) NOT NULL,
    "TargetAmount" numeric(18,2) NOT NULL,
    "CurrentAmount" numeric(18,2) NOT NULL,
    "TargetDate" timestamp with time zone,
    "Color" character varying(32) NOT NULL,
    CONSTRAINT "PK_Goals" PRIMARY KEY ("Id")
);

CREATE UNIQUE INDEX "IX_Users_Email" ON "Users" USING btree ("Email");

CREATE INDEX "IX_Accounts_UserId" ON "Accounts" USING btree ("UserId");

CREATE INDEX "IX_Categories_UserId" ON "Categories" USING btree ("UserId");

CREATE INDEX "IX_Transactions_UserId_Date" ON "Transactions" USING btree ("UserId", "Date");

CREATE INDEX "IX_Transactions_AccountId" ON "Transactions" USING btree ("AccountId");

CREATE INDEX "IX_RecurringRules_UserId_NextRunDate" ON "RecurringRules" USING btree ("UserId", "NextRunDate");

CREATE INDEX "IX_Budgets_UserId_Month" ON "Budgets" USING btree ("UserId", "Month");

CREATE INDEX "IX_Goals_UserId" ON "Goals" USING btree ("UserId");

-- migrate:down

DROP TABLE IF EXISTS "Goals";
DROP TABLE IF EXISTS "Budgets";
DROP TABLE IF EXISTS "RecurringRules";
DROP TABLE IF EXISTS "Transactions";
DROP TABLE IF EXISTS "Categories";
DROP TABLE IF EXISTS "Accounts";
DROP TABLE IF EXISTS "Users";

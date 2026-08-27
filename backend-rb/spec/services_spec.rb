# frozen_string_literal: true

require_relative "../lib/finance"

RSpec.describe Services::Balance do
  CHECKING = "00000000-0000-0000-0000-000000000001"
  SAVINGS = "00000000-0000-0000-0000-000000000002"

  def slice(type, amount, account: CHECKING, transfer: nil)
    Repositories::Slice.new(
      account_id: account, transfer_account_id: transfer, category_id: nil,
      type: Domain::Enums.of("TransactionType", type),
      amount: Domain::Money.parse(amount),
      date: Domain::Instant.from_time(Time.utc(2026, 8, 1))
    )
  end

  it "credits the account for income" do
    expect(described_class.delta_for(CHECKING, slice(0, "100")).to_s).to eq("100")
  end

  it "debits the account for an expense" do
    expect(described_class.delta_for(CHECKING, slice(1, "40")).to_s).to eq("-40")
  end

  it "debits the source of a transfer" do
    expect(described_class.delta_for(CHECKING, slice(2, "25", transfer: SAVINGS)).to_s).to eq("-25")
  end

  it "credits the destination of a transfer" do
    expect(described_class.delta_for(SAVINGS, slice(2, "25", transfer: SAVINGS)).to_s).to eq("25")
  end

  it "leaves an unrelated account untouched" do
    expect(described_class.delta_for(SAVINGS, slice(0, "100")).to_s).to eq("0")
  end

  it "treats a transfer as net-worth neutral" do
    expect(described_class.net_worth_delta(slice(2, "25", transfer: SAVINGS)).to_s).to eq("0")
  end

  it "follows income and expense for net worth" do
    expect(described_class.net_worth_delta(slice(0, "100")).to_s).to eq("100")
    expect(described_class.net_worth_delta(slice(1, "40")).to_s).to eq("-40")
  end

  it "starts from the opening amount" do
    account = Repositories::Account.new(
      id: CHECKING, user_id: "u", name: "Checking",
      type: Domain::Enums.of("AccountType", 0),
      initial_balance: Domain::Money.parse("1000.00"), currency: "USD",
      is_archived: false, created_at: Domain::Instant.now
    )
    slices = [ slice(0, "3000"), slice(1, "42.50"), slice(2, "250.75", transfer: SAVINGS) ]
    expect(described_class.balance_of(account, slices).to_s).to eq("3706.75")
  end
end

RSpec.describe "Services.compare_guid" do
  it "compares the first group as signed, not as bytes" do
    # 0x80... sorts before 0x7f... under .NET's Guid.CompareTo.
    high = "80000000-0000-0000-0000-000000000000"
    low = "7f000000-0000-0000-0000-000000000000"
    expect(Services.compare_guid(high, low)).to be < 0
  end

  it "falls through to the trailing bytes" do
    a = "00000000-0000-0000-0000-000000000001"
    b = "00000000-0000-0000-0000-000000000002"
    expect(Services.compare_guid(a, b)).to be < 0
    expect(Services.compare_guid(a, a)).to eq(0)
  end
end

RSpec.describe Services::RecurringService do
  def rule(**overrides)
    Repositories::RecurringRule.new(
      { id: "r", user_id: "u", account_id: "a", category_id: nil,
        type: Domain::Enums.of("TransactionType", 1),
        amount: Domain::Money.parse("9.99"), description: "Streaming",
        frequency: Domain::Enums.of("Frequency", 2),
        start_date: Domain::Instant.from_time(Time.utc(2026, 1, 1)),
        end_date: nil,
        next_run_date: Domain::Instant.from_time(Time.utc(2026, 1, 1)),
        is_active: true }.merge(overrides)
    )
  end

  def at(time) = Domain::Instant.from_time(time)

  {
    0 => Time.utc(2026, 1, 2), 1 => Time.utc(2026, 1, 8),
    2 => Time.utc(2026, 2, 1), 3 => Time.utc(2027, 1, 1)
  }.each do |frequency, expected|
    it "steps by frequency #{frequency}" do
      next_run = described_class.advance(Time.utc(2026, 1, 1), Domain::Enums.of("Frequency", frequency))
      expect(next_run).to eq(expected)
    end
  end

  it "clamps the day when stepping a month" do
    expect(described_class.advance(Time.utc(2026, 1, 31), Domain::Enums.of("Frequency", 2)))
      .to eq(Time.utc(2026, 2, 28))
  end

  it "creates one per period up to the cutoff" do
    target = rule
    created = described_class.materialize(target, at(Time.utc(2026, 3, 15)))
    expect(created.map { |item| item.date.to_s[0, 10] }).to eq(%w[2026-01-01 2026-02-01 2026-03-01])
    expect(target.next_run_date.to_s).to eq("2026-04-01T00:00:00Z")
  end

  it "creates nothing before the first run" do
    target = rule(next_run_date: at(Time.utc(2026, 6, 1)))
    expect(described_class.materialize(target, at(Time.utc(2026, 1, 1)))).to eq([])
  end

  it "tags what it creates" do
    created = described_class.materialize(rule, at(Time.utc(2026, 1, 2)))
    expect(created.first.tags).to eq([ "recurring" ])
    expect(created.first.notes).to be_nil
    expect(created.first.transfer_account_id).to be_nil
  end

  it "still runs an occurrence exactly on the end date" do
    target = rule(end_date: at(Time.utc(2026, 2, 1)))
    expect(described_class.materialize(target, at(Time.utc(2026, 6, 1))).length).to eq(2)
    expect(target.is_active).to be(false)
  end

  it "retires a rule past its end" do
    target = rule(end_date: at(Time.utc(2025, 12, 1)))
    expect(described_class.materialize(target, at(Time.utc(2026, 6, 1)))).to eq([])
    expect(target.is_active).to be(false)
  end

  it "caps a long-dormant rule and leaves it running" do
    target = rule(frequency: Domain::Enums.of("Frequency", 0))
    created = described_class.materialize(target, at(Time.utc(2030, 1, 1)))
    expect(created.length).to eq(described_class::MAX_OCCURRENCES_PER_PASS)
    expect(target.is_active).to be(true)
  end

  it "produces nothing for an inactive rule" do
    expect(described_class.materialize(rule(is_active: false), at(Time.utc(2030, 1, 1)))).to eq([])
  end
end

RSpec.describe Services::CsvService do
  it "reads plain rows" do
    expect(described_class.parse("a,b\n1,2\n")).to eq([ %w[a b], %w[1 2] ])
  end

  it "adds no empty row for a trailing newline" do
    expect(described_class.parse("a,b\n1,2\n\n").length).to eq(2)
  end

  it "handles quoted commas and doubled quotes" do
    expect(described_class.parse(%("a,b","say ""hi"""\n))).to eq([ [ "a,b", 'say "hi"' ] ])
  end

  it "lets a quoted field span lines" do
    expect(described_class.parse(%("line1\nline2",x\n))).to eq([ [ "line1\nline2", "x" ] ])
  end

  it "ignores carriage returns outside quotes" do
    expect(described_class.parse("a,b\r\n1,2\r\n")).to eq([ %w[a b], %w[1 2] ])
  end

  it "reads empty input as no rows" do
    expect(described_class.parse("")).to eq([])
  end

  {
    "plain" => "plain", "has,comma" => %("has,comma"),
    %(has"quote) => %("has""quote"), "has\nnewline" => %("has\nnewline")
  }.each do |value, expected|
    it "quotes #{value.inspect} only when needed" do
      expect(described_class.escape_field(value)).to eq(expected)
    end
  end

  {
    "12.50" => "12.50", "$12.50" => "12.50", "1,234.56" => "1234.56",
    "(12.50)" => "-12.50", "12.50-" => "-12.50"
  }.each do |value, expected|
    it "reads the currency shape #{value}" do
      expect(described_class.parse_currency(value).to_s).to eq(expected)
    end
  end

  it "rejects nonsense currency" do
    expect(described_class.parse_currency("abc")).to be_nil
  end

  [ "2026-08-26", "2026-08-26T10:00:00", "2026-08-26 10:00:00", "08/26/2026", "8/26/2026" ]
    .each do |value|
    it "reads the import date layout #{value}" do
      expect(described_class.parse_date(value)).not_to be_nil
    end
  end

  it "reads a PM marker as afternoon" do
    expect(described_class.parse_date("8/26/2026 3:04:05 PM").to_s).to eq("2026-08-26T15:04:05Z")
  end

  it "rejects a day-first date rather than rolling it over" do
    # Time.utc would happily turn a 26th month into 2028.
    expect(described_class.parse_date("26/08/2026")).to be_nil
  end
end

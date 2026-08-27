# frozen_string_literal: true

require_relative "../lib/finance"

M = Domain::Money

def money(text) = M.parse(text)

RSpec.describe Domain::Money do
  it "keeps the scale it was written with" do
    expect(money("1250.00").to_s).to eq("1250.00")
    expect(money("12.5").to_s).to eq("12.5")
    expect(money("10").to_s).to eq("10")
    expect(money("-0.50").to_s).to eq("-0.50")
  end

  it "adds with the wider of the two scales" do
    expect(money("1.5").add(money("2.25")).to_s).to eq("3.75")
    expect(money("10").add(money("0.01")).to_s).to eq("10.01")
  end

  it "subtracts past zero" do
    expect(money("100.00").subtract(money("250.75")).to_s).to eq("-150.75")
  end

  {
    "1.005" => "1.01", "-1.005" => "-1.01", "2.344" => "2.34", "2.345" => "2.35",
    # Never lengthens the scale.
    "10" => "10", "0.1" => "0.1"
  }.each do |input, expected|
    it "rounds #{input} to #{expected}, half away from zero" do
      expect(money(input).round_money.to_s).to eq(expected)
    end
  end

  it "renders exactly two places for the CSV column" do
    expect(money("12.5").to_fixed2).to eq("12.50")
    expect(money("3000").to_fixed2).to eq("3000.00")
  end

  it "drops trailing zeros when trimmed" do
    expect(money("0.7500").trim.to_s).to eq("0.75")
    expect(money("0.5000").trim.to_s).to eq("0.5")
  end

  it "compares across scales" do
    expect(money("1.50")).to eq(money("1.5"))
    expect(money("1.5")).to be < money("1.51")
  end

  it "divides to a fixed number of places" do
    expect(money("2250").divide(money("3000"), 4).trim.to_s).to eq("0.75")
  end

  %w[abc 1e5 1,000 --1].each do |bad|
    it "rejects #{bad}" do
      expect(M.parse(bad)).to be_nil
    end
  end
end

RSpec.describe Api::Rendering do
  it "writes money as a bare number with its scale" do
    expect(described_class.dump({ "balance" => money("1250.00") })).to eq('{"balance":1250.00}')
  end

  it "writes a defined enum as its camelCase name" do
    expect(described_class.dump({ "type" => Domain::Enums.of("AccountType", 2) }))
      .to eq('{"type":"creditCard"}')
  end

  it "writes an undefined ordinal as a bare number" do
    # The .NET converter round-trips a value naming no member.
    expect(described_class.dump({ "type" => Domain::Enums.of("AccountType", 99) }))
      .to eq('{"type":99}')
  end

  it "escapes text so it cannot forge a number" do
    rendered = described_class.dump({ "text" => %q(1,"balance":999) })
    expect(JSON.parse(rendered)).to eq({ "text" => %q(1,"balance":999) })
  end
end

RSpec.describe Domain::Instant do
  it "keeps microseconds a second-precision format would drop" do
    expect(described_class.from_pg("2026-08-26 22:27:26.51655+00").to_s)
      .to eq("2026-08-26T22:27:26.51655Z")
  end

  it "omits the fraction when it is zero" do
    expect(described_class.from_pg("2026-08-26 22:27:26+00").to_s).to eq("2026-08-26T22:27:26Z")
  end

  it "applies a non-UTC offset" do
    expect(described_class.from_pg("2026-08-26 20:00:00+02").to_s).to eq("2026-08-26T18:00:00Z")
  end

  [ "2026-08-26", "2026-08-26T10:00", "2026-08-26T10:00:00", "2026-08-26T10:00:00Z" ].each do |value|
    it "parses the wire layout #{value}" do
      expect(described_class.parse_wire(value)).not_to be_nil
    end
  end

  it "reads a naive value as UTC" do
    expect(described_class.parse_wire("2026-08-26T10:00:00").to_s).to eq("2026-08-26T10:00:00Z")
  end

  it "rejects nonsense" do
    expect(described_class.parse_wire("last tuesday")).to be_nil
  end
end

RSpec.describe Domain::Dates do
  it "clamps rather than rolling over" do
    jan31 = Time.utc(2026, 1, 31)
    expect(described_class.add_months(jan31, 1)).to eq(Time.utc(2026, 2, 28))
    # Clamping is not remembered: Feb 28 + 1 month is Mar 28, not Mar 31.
    expect(described_class.add_months(described_class.add_months(jan31, 1), 1))
      .to eq(Time.utc(2026, 3, 28))
  end

  it "walks backwards across a year boundary" do
    expect(described_class.add_months(Time.utc(2026, 1, 15), -1)).to eq(Time.utc(2025, 12, 15))
  end

  it "clamps a leap day in a common year" do
    expect(described_class.add_years(Time.utc(2028, 2, 29), 1)).to eq(Time.utc(2029, 2, 28))
  end

  it "builds a window ending on the reference month" do
    window = described_class.trailing_months(Time.utc(2026, 3, 15), 3)
    expect(window.map { |m| described_class.month_key(m) }).to eq(%w[2026-01 2026-02 2026-03])
  end

  [ "2026-13", "nope", "", "2026" ].each do |bad|
    it "rejects the month #{bad.inspect}" do
      expect(described_class.try_parse_month(bad)).to be_nil
    end
  end
end

RSpec.describe Domain::Enums do
  %w[creditCard CREDITCARD creditcard].each do |name|
    it "parses #{name} in any casing" do
      expect(described_class.parse("AccountType", name).ordinal).to eq(2)
    end
  end

  it "parses an ordinal as a number or digits" do
    expect(described_class.parse("TransactionType", 2).ordinal).to eq(2)
    expect(described_class.parse("TransactionType", "2").ordinal).to eq(2)
  end

  it "preserves an undefined ordinal" do
    parsed = described_class.parse("AccountType", 99)
    expect(parsed.ordinal).to eq(99)
    expect(parsed.defined?).to be(false)
    expect(parsed.wire_name).to eq("99")
  end

  it "rejects an unknown name" do
    expect(described_class.parse("AccountType", "rust")).to be_nil
  end
end

RSpec.describe Domain::Tags do
  it "round-trips" do
    expect(described_class.split(described_class.join(%w[food out]))).to eq(%w[food out])
  end

  it "trims and drops blanks" do
    expect(described_class.split(described_class.join([ " food ", "  ", "", "out" ])))
      .to eq(%w[food out])
  end

  it "collapses whitespace-only notes to nil" do
    expect(described_class.trimmed_or_nil("   ")).to be_nil
    expect(described_class.trimmed_or_nil(" hi ")).to eq("hi")
  end
end

RSpec.describe Domain::Validation do
  it "puts the field name first for required" do
    errs = Domain::Errors::FieldErrors.new
    described_class.required(errs, "Email", "   ")
    expect(errs.errors).to eq({ "Email" => [ "The Email field is required." ] })
  end

  it "puts 'The field' first for a length rule" do
    errs = Domain::Errors::FieldErrors.new
    described_class.max_length(errs, "Name", "n" * 201, 200)
    expect(errs.errors["Name"]).to eq(
      [ "The field Name must be a string or array type with a maximum length of '200'." ]
    )
  end

  it "quotes the money bounds verbatim" do
    errs = Domain::Errors::FieldErrors.new
    described_class.money_range(errs, "Amount", money("0"), "0.01")
    expect(errs.errors["Amount"]).to eq(
      [ "The field Amount must be between 0.01 and 999999999999.99." ]
    )
  end

  { "a@b.c" => true, "bad" => false, "@b.c" => false, "a@" => false, "a@b@c" => false }
    .each do |value, valid|
    it "checks the email shape of #{value}" do
      errs = Domain::Errors::FieldErrors.new
      described_class.email_address(errs, "Email", value)
      expect(errs.empty?).to be(valid)
    end
  end

  it "short-circuits on a missing required member" do
    errs = Domain::Errors::FieldErrors.new
    expect(described_class.required_members(errs, %w[accountId type])).to be(true)
    expect(errs.errors["$"]).to eq(
      [ "The JSON payload was missing required properties, including the following: accountId, type" ]
    )
  end
end

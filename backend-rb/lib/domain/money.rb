# frozen_string_literal: true

module Domain
  # Money, carried as an integer count of the smallest unit plus the scale it
  # holds.
  #
  # BigDecimal will not do: BigDecimal("1250.00").to_s("F") is "1250.0", and the
  # API has to put 1250.00 on the wire because the other four backends do.
  # Arithmetic follows the same rule their decimal libraries use -- an addition
  # keeps the wider of the two scales.
  class Money
    include Comparable

    SCALE = 2

    # The bounds every [Range(typeof(decimal), ...)] in the DTOs uses.
    MIN_POSITIVE = "0.01"
    MIN_ZERO = "0.00"
    MAX = "999999999999.99"

    LITERAL = /\A[+-]?\d+(\.\d+)?\z/

    attr_reader :units, :scale

    def initialize(units, scale)
      @units = units
      @scale = scale
      freeze
    end

    class << self
      def zero = new(0, 0)

      # Reads a decimal literal, keeping the scale it was written with.
      def parse(text)
        trimmed = text.to_s.strip
        return nil unless LITERAL.match?(trimmed)

        negative = trimmed.start_with?("-")
        digits = trimmed.sub(/\A[+-]/, "")
        whole, fraction = digits.split(".")
        fraction ||= ""

        units = "#{whole}#{fraction}".to_i
        new(negative ? -units : units, fraction.length)
      end

      # Reads whatever JSON produced: a number, or a quoted number.
      def from_json(value)
        case value
        when ::Integer then new(value, 0)
        when ::Float, ::BigDecimal then parse(format("%.10f", value).sub(/0+\z/, "").sub(/\.\z/, ""))
        when ::String then parse(value)
        else nil # null, a bool, an object -- not a number the caller can use
        end
      end

      # Parses one of the bound constants above, which are always well-formed.
      def bound(text) = parse(text) || raise(ArgumentError, "money: #{text} is not a decimal")
    end

    def add(other)
      left, right, scale = align(other)
      self.class.new(left + right, scale)
    end

    def subtract(other)
      left, right, scale = align(other)
      self.class.new(left - right, scale)
    end

    def negate = self.class.new(-units, scale)

    def <=>(other)
      left, right, = align(other)
      left <=> right
    end

    def zero? = units.zero?

    # Rounds to two places, half away from zero -- and never lengthens the
    # scale, so 10 stays 10 rather than becoming 10.00.
    def round_money
      return self if scale <= SCALE

      divisor = 10**(scale - SCALE)
      magnitude = units.abs
      quotient, remainder = magnitude.divmod(divisor)
      rounded = remainder * 2 >= divisor ? quotient + 1 : quotient

      self.class.new(units.negative? ? -rounded : rounded, SCALE)
    end

    # Drops trailing fractional zeros. Used only on the savings rate.
    def trim
      value = units
      places = scale
      while places.positive? && (value % 10).zero?
        value /= 10
        places -= 1
      end
      self.class.new(value, places)
    end

    # Divides to a fixed number of places, rounding half away from zero. Money
    # has no general division because the API needs it in exactly one place.
    def divide(other, places)
      left, right, = align(other)
      return self.class.zero if right.zero?

      negative = left.negative? ^ right.negative?
      scaled = left.abs * (10**places)
      quotient, remainder = scaled.divmod(right.abs)
      rounded = remainder * 2 >= right.abs ? quotient + 1 : quotient

      self.class.new(negative ? -rounded : rounded, places)
    end

    # Renders as a bare decimal literal, keeping its scale.
    def to_s
      return units.to_s if scale.zero?

      digits = units.abs.to_s.rjust(scale + 1, "0")
      whole = digits[0...-scale]
      fraction = digits[-scale..]
      "#{units.negative? ? '-' : ''}#{whole}.#{fraction}"
    end

    # Exactly two places, whatever the scale. The CSV column wants this.
    def to_fixed2
      return to_s if scale == SCALE
      return rescale(SCALE).to_s if scale < SCALE

      round_money.to_s
    end

    def as_json(*) = self
    def inspect = "#<Money #{self}>"

    private

    def align(other)
      common = [ scale, other.scale ].max
      [ units * 10**(common - scale), other.units * 10**(common - other.scale), common ]
    end

    def rescale(places) = self.class.new(units * 10**(places - scale), places)
  end

  MONEY_ZERO = Money.zero
end

module Domain
  # Tag packing for the TagsRaw column.
  module Tags
    # US, the unit separator: it cannot occur inside a tag. Built from its
    # codepoint rather than written literally, because a raw control byte in
    # source is invisible in review and easy to mangle when copied.
    SEPARATOR = 31.chr(Encoding::UTF_8)

    module_function

    # Trims, drops blanks, and packs into the storage column.
    def join(tags)
      Array(tags).map { |tag| tag.to_s.strip }.reject(&:empty?).join(SEPARATOR)
    end

    # Unpacks the storage column. Always an array, never nil.
    def split(raw) = raw.to_s.split(SEPARATOR).reject(&:empty?)

    # Whitespace-only collapses to nil, as the other backends do for Notes.
    def trimmed_or_nil(value)
      text = value.to_s.strip
      text.empty? ? nil : text
    end
  end
end

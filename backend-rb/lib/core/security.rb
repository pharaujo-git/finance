# frozen_string_literal: true

require "openssl"
require "securerandom"
require "base64"
require "jwt"

module Core
  # The two things all five backends must agree on byte for byte: the ASP.NET
  # Core Identity v3 password blob and the HS256 bearer token.
  #
  # A token minted here must be accepted by the .NET, Go, Python and Node APIs
  # and vice versa, so none of the constants below are free to change.
  module Security
    # Layout of the base64-encoded version-3 blob:
    #
    #   byte  0      format marker, always 0x01
    #   bytes 1..4   PRF id, uint32 big-endian
    #   bytes 5..8   iteration count, uint32 big-endian
    #   bytes 9..12  salt length in bytes, uint32 big-endian
    #   bytes 13..   salt, then the derived subkey (rest of the blob)
    FORMAT_MARKER_V3 = 0x01
    HEADER_LEN = 13

    # PRF 0 and 1 are only ever read from blobs written before the defaults moved.
    PRF_DIGESTS = { 0 => "SHA1", 1 => "SHA256", 2 => "SHA512" }.freeze

    DEFAULT_PRF = 2
    DEFAULT_ITERATIONS = 100_000
    DEFAULT_SALT_LEN = 16
    DEFAULT_SUBKEY_LEN = 32

    # Identity rejects salts under 128 bits, and treats absurd counts as
    # corruption rather than working through them.
    MIN_SALT_LEN = 16
    MAX_ITERATIONS = 10_000_000

    # Issuer, audience and the 7-day lifetime match JwtOptions in the .NET API.
    ISSUER = "finance-tracker"
    AUDIENCE = "finance-tracker"
    TOKEN_LIFETIME_SECONDS = 7 * 24 * 60 * 60
    # Matches the JwtBearer ClockSkew of one minute.
    CLOCK_LEEWAY_SECONDS = 60
    SIGNING_ALG = "HS256"

    UUID_PATTERN = /\A[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z/i

    # Raised for every rejection reason, so callers cannot leak which check failed.
    class InvalidToken < StandardError; end

    Principal = Struct.new(:user_id, :email)

    module_function

    # Derives a new Identity v3 blob using the current defaults.
    def hash_password(password)
      salt = SecureRandom.bytes(DEFAULT_SALT_LEN)
      subkey = OpenSSL::PKCS5.pbkdf2_hmac(
        password, salt, DEFAULT_ITERATIONS, DEFAULT_SUBKEY_LEN,
        OpenSSL::Digest.new(PRF_DIGESTS.fetch(DEFAULT_PRF))
      )

      header = [ FORMAT_MARKER_V3 ].pack("C") +
               [ DEFAULT_PRF, DEFAULT_ITERATIONS, DEFAULT_SALT_LEN ].pack("N3")
      Base64.strict_encode64(header + salt + subkey)
    end

    # Checks a password against a stored blob. A correct password whose blob
    # uses parameters weaker than the current defaults reports
    # :success_rehash_needed, exactly as Identity's own hasher does.
    def verify_password(stored_hash, password)
      blob = decode_blob(stored_hash)
      return :failed if blob.nil?
      return :failed if blob.bytesize < HEADER_LEN + 1
      return :failed unless blob.getbyte(0) == FORMAT_MARKER_V3

      prf_id, iterations, salt_len = blob.byteslice(1, 12).unpack("N3")

      # Both bounds also keep the slicing below in range.
      return :failed if salt_len < MIN_SALT_LEN || HEADER_LEN + salt_len >= blob.bytesize
      return :failed if iterations.zero? || iterations > MAX_ITERATIONS

      digest = PRF_DIGESTS[prf_id]
      return :failed if digest.nil?

      salt = blob.byteslice(HEADER_LEN, salt_len)
      expected = blob.byteslice(HEADER_LEN + salt_len..)
      return :failed if expected.bytesize < 16

      actual = OpenSSL::PKCS5.pbkdf2_hmac(
        password, salt, iterations, expected.bytesize, OpenSSL::Digest.new(digest)
      )
      return :failed unless OpenSSL.secure_compare(actual, expected)

      if prf_id != DEFAULT_PRF || iterations < DEFAULT_ITERATIONS ||
         expected.bytesize < DEFAULT_SUBKEY_LEN
        return :success_rehash_needed
      end

      :success
    end

    def decode_blob(stored_hash)
      text = stored_hash.to_s
      return nil if text.empty?

      Base64.strict_decode64(text)
    rescue ArgumentError
      nil
    end

    # Issues and validates tokens interchangeable with the other four APIs.
    class TokenService
      def initialize(secret)
        @secret = secret
      end

      # Mints a token. The claim set matches what the other backends write.
      def issue(user_id, email, now = Time.now)
        issued_at = now.to_i
        JWT.encode(
          {
            iss: ISSUER, aud: AUDIENCE,
            exp: issued_at + TOKEN_LIFETIME_SECONDS,
            iat: issued_at, nbf: issued_at,
            sub: user_id, email: email, jti: SecureRandom.uuid
          },
          @secret, SIGNING_ALG
        )
      end

      # Checks signature, issuer, audience and lifetime, then extracts the caller.
      def validate(token)
        claims, = JWT.decode(
          token, @secret, true,
          algorithm: SIGNING_ALG, iss: ISSUER, verify_iss: true,
          aud: AUDIENCE, verify_aud: true,
          verify_expiration: true, required_claims: %w[exp sub],
          leeway: CLOCK_LEEWAY_SECONDS
        )

        subject = claims["sub"].to_s
        raise InvalidToken, "subject is not a uuid" unless UUID_PATTERN.match?(subject)

        Principal.new(subject, claims["email"].is_a?(String) ? claims["email"] : "")
      rescue JWT::DecodeError, JWT::VerificationError => e
        raise InvalidToken, e.message
      end
    end
  end
end

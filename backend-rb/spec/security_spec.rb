# frozen_string_literal: true

require_relative "../lib/finance"

# The identity layer, which has to interoperate with the other four APIs.
RSpec.describe Core::Security do
  SECRET = "finance-tracker-local-development-signing-key-please-override"
  USER_ID = "3d4d6087-3456-450e-8392-3887ad469b95"

  # Builds an Identity v3 blob with chosen parameters.
  def blob(password, prf:, iterations:, salt_len:, subkey_len:)
    digests = { 0 => "SHA1", 1 => "SHA256", 2 => "SHA512" }
    salt = "\x01".b * salt_len
    subkey = OpenSSL::PKCS5.pbkdf2_hmac(password, salt, iterations, subkey_len,
                                        OpenSSL::Digest.new(digests[prf]))
    header = [ 1 ].pack("C") + [ prf, iterations, salt_len ].pack("N3")
    Base64.strict_encode64(header + salt + subkey)
  end

  describe "password hashing" do
    it "round-trips" do
      expect(described_class.verify_password(described_class.hash_password("Passw0rd!123"),
                                             "Passw0rd!123")).to eq(:success)
    end

    it "rejects a wrong password" do
      expect(described_class.verify_password(described_class.hash_password("Passw0rd!123"),
                                             "Passw0rd!124")).to eq(:failed)
    end

    it "writes the current defaults" do
      raw = Base64.strict_decode64(described_class.hash_password("x"))
      prf, iterations, salt_len = raw.byteslice(1, 12).unpack("N3")
      expect(raw.getbyte(0)).to eq(1)
      expect(prf).to eq(described_class::DEFAULT_PRF)
      expect(iterations).to eq(described_class::DEFAULT_ITERATIONS)
      expect(salt_len).to eq(16)
      expect(raw.bytesize).to eq(13 + 16 + 32)
    end

    it "uses a fresh salt each time" do
      first = described_class.hash_password("same")
      second = described_class.hash_password("same")
      expect(first).not_to eq(second)
    end

    it "accepts a weaker blob but asks for a rehash" do
      # A blob the .NET API would have written years ago.
      stored = blob("legacy", prf: 1, iterations: 10_000, salt_len: 16, subkey_len: 32)
      expect(described_class.verify_password(stored, "legacy")).to eq(:success_rehash_needed)
    end

    it "flags a short subkey for rehash" do
      stored = blob("legacy", prf: 2, iterations: described_class::DEFAULT_ITERATIONS,
                    salt_len: 16, subkey_len: 16)
      expect(described_class.verify_password(stored, "legacy")).to eq(:success_rehash_needed)
    end

    [ "not base64 at all!!", "", Base64.strict_encode64("\x02".b + ("\x00".b * 40)) ].each do |stored|
      it "rejects the malformed blob #{stored[0, 12].inspect}" do
        expect(described_class.verify_password(stored, "anything")).to eq(:failed)
      end
    end

    it "rejects a salt below 128 bits" do
      stored = blob("x", prf: 2, iterations: 1000, salt_len: 8, subkey_len: 32)
      expect(described_class.verify_password(stored, "x")).to eq(:failed)
    end

    it "rejects an absurd iteration count" do
      raw = Base64.strict_decode64(described_class.hash_password("x")).dup
      raw[5, 4] = [ 20_000_000 ].pack("N")
      expect(described_class.verify_password(Base64.strict_encode64(raw), "x")).to eq(:failed)
    end
  end

  describe Core::Security::TokenService do
    let(:service) { described_class.new(SECRET) }

    it "round-trips" do
      principal = service.validate(service.issue(USER_ID, "owner@example.com"))
      expect(principal.user_id).to eq(USER_ID)
      expect(principal.email).to eq("owner@example.com")
    end

    it "writes the shared claim set" do
      claims, = JWT.decode(service.issue(USER_ID, "owner@example.com"), SECRET, true,
                           algorithm: "HS256", aud: "finance-tracker", verify_aud: true)
      expect(claims["iss"]).to eq("finance-tracker")
      expect(claims["aud"]).to eq("finance-tracker")
      expect(claims["sub"]).to eq(USER_ID)
      expect(claims["exp"] - claims["iat"]).to eq(7 * 24 * 3600)
      expect(claims["nbf"]).to eq(claims["iat"])
    end

    it "rejects another secret" do
      issued = service.issue(USER_ID, "a@b.c")
      expect { described_class.new("a different signing key").validate(issued) }
        .to raise_error(Core::Security::InvalidToken)
    end

    it "rejects a tampered payload" do
      header, payload, signature = service.issue(USER_ID, "a@b.c").split(".")
      forged = "#{header}.#{payload[0..-5]}AAAA.#{signature}"
      expect { service.validate(forged) }.to raise_error(Core::Security::InvalidToken)
    end

    it "rejects an expired token" do
      long_ago = Time.now - (8 * 24 * 3600)
      expect { service.validate(service.issue(USER_ID, "a@b.c", long_ago)) }
        .to raise_error(Core::Security::InvalidToken)
    end

    it "rejects the none algorithm" do
      forged = JWT.encode({ sub: USER_ID, iss: "finance-tracker", aud: "finance-tracker",
                            exp: Time.now.to_i + 3600 }, nil, "none")
      expect { service.validate(forged) }.to raise_error(Core::Security::InvalidToken)
    end

    it "rejects a foreign issuer" do
      forged = JWT.encode({ sub: USER_ID, iss: "somewhere-else", aud: "finance-tracker",
                            exp: Time.now.to_i + 3600 }, SECRET, "HS256")
      expect { service.validate(forged) }.to raise_error(Core::Security::InvalidToken)
    end

    it "rejects a non-uuid subject" do
      forged = JWT.encode({ sub: "not-a-uuid", iss: "finance-tracker", aud: "finance-tracker",
                            exp: Time.now.to_i + 3600 }, SECRET, "HS256")
      expect { service.validate(forged) }.to raise_error(Core::Security::InvalidToken)
    end
  end
end

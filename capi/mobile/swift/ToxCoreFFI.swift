import Foundation

// Thin Swift wrapper for toxcore C ABI.
public enum ToxABI {
    public static let featureGenerateKeypair: UInt64 = 1 << 0
    public static let featureSecureWipe: UInt64 = 1 << 1
    public static let featureSafetyNumber: UInt64 = 1 << 2

    public static func versionTuple() -> (UInt32, UInt32, UInt32) {
        (tox_abi_version_major(), tox_abi_version_minor(), tox_abi_version_patch())
    }

    public static func versionString() -> String {
        let needed = Int(tox_abi_version_string(nil, 0))
        guard needed > 0 else { return "" }

        var buf = [CChar](repeating: 0, count: needed + 1)
        _ = buf.withUnsafeMutableBufferPointer { ptr in
            tox_abi_version_string(UnsafeMutableRawPointer(ptr.baseAddress)?.assumingMemoryBound(to: UInt8.self), ptr.count)
        }
        return String(cString: buf)
    }

    public static func featureFlags() -> UInt64 {
        UInt64(tox_abi_feature_flags())
    }

    public static func hasRequiredSecurityPrimitives() -> Bool {
        let required = featureGenerateKeypair | featureSecureWipe | featureSafetyNumber
        return (featureFlags() & required) == required
    }
}

public enum ToxCryptoFFI {
    public static func generateKeypair() -> (publicKey: [UInt8], secretKey: [UInt8])? {
        var pub = [UInt8](repeating: 0, count: 32)
        var sec = [UInt8](repeating: 0, count: 32)
        let ok = pub.withUnsafeMutableBufferPointer { pubPtr in
            sec.withUnsafeMutableBufferPointer { secPtr in
                tox_crypto_generate_keypair(pubPtr.baseAddress, secPtr.baseAddress)
            }
        }
        return ok == 1 ? (pub, sec) : nil
    }

    public static func secureWipe(_ bytes: inout [UInt8]) -> Bool {
        if bytes.isEmpty { return true }
        return bytes.withUnsafeMutableBufferPointer { ptr in
            tox_crypto_secure_wipe(ptr.baseAddress, ptr.count) == 1
        }
    }
}

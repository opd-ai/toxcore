package org.toxcore.mobile

// Thin Kotlin/JNI wrapper for toxcore C ABI.
object ToxABI {
    const val FEATURE_GENERATE_KEYPAIR: Long = 1L shl 0
    const val FEATURE_SECURE_WIPE: Long = 1L shl 1
    const val FEATURE_SAFETY_NUMBER: Long = 1L shl 2

    external fun abiVersionMajor(): Int
    external fun abiVersionMinor(): Int
    external fun abiVersionPatch(): Int
    external fun abiVersionString(): String
    external fun abiFeatureFlags(): Long

    fun hasRequiredSecurityPrimitives(): Boolean {
        val required = FEATURE_GENERATE_KEYPAIR or FEATURE_SECURE_WIPE or FEATURE_SAFETY_NUMBER
        return (abiFeatureFlags() and required) == required
    }
}

object ToxCryptoFFI {
    external fun generateKeypair(publicKeyOut: ByteArray, secretKeyOut: ByteArray): Boolean
    external fun secureWipe(data: ByteArray): Boolean

    fun generateKeypair(): Pair<ByteArray, ByteArray>? {
        val publicKey = ByteArray(32)
        val secretKey = ByteArray(32)
        if (!generateKeypair(publicKey, secretKey)) {
            return null
        }
        return Pair(publicKey, secretKey)
    }
}

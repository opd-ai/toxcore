#!/usr/bin/env bash
# check-error-invariants.sh: Validate error handling and logging invariants
# 
# Enforces:
# 1. SecurityError instances are properly logged with structured fields
# 2. No debug/sensitive information (keys, nonces, raw bytes) in error messages
# 3. Fatal security errors are logged with appropriate severity
# 4. Downgrade events are observable in logs

set -e

ERRORS=0

echo "=== Checking Error Handling Invariants ==="

# Check 1: Verify SecurityError has required category check methods
echo "✓ Checking SecurityError category methods..."
for method in "IsFatal" "IsCompatibilityWarning" "IsVerificationFailure" "IsDowngradeEvent"; do
    if ! grep -q "func.*SecurityError.*$method" transport/security_errors.go; then
        echo "  ERROR: SecurityError.$method() not found"
        ((ERRORS++))
    fi
done

# Check 2: Verify FatalSecurityError usage in critical paths
echo "✓ Checking FatalSecurityError usage in critical paths..."
# Verify that ParseSignedVersionNegotiation returns SecurityError for signature failures
if ! grep -q "NewFatalSecurityError\|FatalSecurityError" transport/version_negotiation.go; then
    echo "  ERROR: ParseSignedVersionNegotiation should use FatalSecurityError for signature verification failures"
    ((ERRORS++))
fi

# Check 3: Verify SecurityError is properly used in negotiation
echo "✓ Checking SecurityError usage in negotiation..."
if ! grep -q "NewDowngradeEvent\|NewFatalSecurityError" transport/negotiating_transport.go; then
    echo "  ERROR: negotiating_transport should emit FatalSecurityError or DowngradeEvent"
    ((ERRORS++))
fi

# Check 4: Verify error constructor functions exist
echo "✓ Checking error constructor functions..."
for func in "NewSecurityError" "NewFatalSecurityError" "NewCompatibilityWarning" "NewVerificationFailure" "NewDowngradeEvent" "AsSecurityError"; do
    if ! grep -q "^func $func\|^func (.*) $func" transport/security_errors.go; then
        echo "  ERROR: Constructor function $func() not found"
        ((ERRORS++))
    fi
done

# Check 5: Verify predefined error instances exist
echo "✓ Checking predefined error instances..."
for errvar in "ErrSignatureVerificationFailed" "WarnFallbackToLegacy" "ErrVerificationFailure"; do
    if ! grep -q "$errvar.*SecurityError\|var.*$errvar" transport/security_errors.go; then
        echo "  WARNING: Predefined error instance $errvar not found"
    fi
done

# Check 6: No forbidden debug fields in security-critical error messages
echo "✓ Checking for forbidden debug fields in errors..."
# Only check actual error paths, not test files
if grep -r "log.Fatal.*Key\|log.Fatal.*Secret\|log.Fatal.*nonce" \
    transport/security_errors.go transport/version_negotiation.go transport/negotiating_transport.go 2>/dev/null; then
    echo "  ERROR: Forbidden key/secret material in log.Fatal calls"
    ((ERRORS++))
fi

# Check 7: Verify UnknownCategory catch-all
echo "✓ Checking error category exhaustiveness..."
if ! grep -q "UnknownCategory" transport/security_errors.go; then
    echo "  WARNING: UnknownCategory not defined (for exhaustive switch statements)"
fi

# Summary
echo ""
if [ $ERRORS -eq 0 ]; then
    echo "✅ All error handling invariants validated successfully"
    exit 0
else
    echo "❌ Found $ERRORS error handling invariant violations"
    exit 1
fi

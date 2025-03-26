# Prompt: Migrating Tox Handshake to Noise-IK Implementation

## Context

The Tox protocol currently uses a custom-built cryptographic handshake mechanism that may be vulnerable to Key Compromise Impersonation (KCI) attacks. We need to migrate to the Noise Protocol Framework's IK pattern using the flynn/noise library to improve security posture.

## Requirements

Design a comprehensive migration plan that:

1. Analyzes the current Tox handshake implementation
2. Maps current functionality to Noise-IK pattern components
3. Implements a solution using flynn/noise library
4. Provides backward compatibility during transition
5. Includes testing strategy to verify security properties

## Technical Approach

Your implementation plan should:

- Document key differences between current handshake and Noise-IK
- Identify all code components requiring modification
- Detail protocol negotiation for compatibility with older clients
- Handle key management transition
- Provide secure fallback mechanisms if handshake fails

## Deliverables

1. Technical specification for new handshake implementation
2. Implementation roadmap with milestones
3. Testing framework for security validation
4. Migration strategy for existing network nodes
5. Performance comparison between old and new implementations

Include considerations for cryptographic agility, forward secrecy, and resistance to KCI attacks in your design.
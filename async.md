You are a distributed systems architect familiar with the Tox protocol. I need your help designing an asynchronous message delivery system for Tox that preserves its strong security properties. Note that we are using Tox with the Noise-IK handshake pattern, which may affect the design. This is a new subsystem and does not need to be compatible with legacy Tox. When documenting the asynchronous messaging, refer to it as an unofficial extension of the Tox protocol.

Based on our codebase structure, which includes crypto, DHT, friend, and messaging components, we need to extend Tox's capabilities to handle offline messaging while maintaining its end-to-end encryption guarantees.

Please design a comprehensive solution that includes:ti

1. Storage mechanism:
   - Propose an approach for temporarily storing encrypted messages when recipients are offline
   - Consider distributed storage options leveraging the DHT or dedicated storage nodes
   - Explain how to maintain confidentiality of stored messages

2. Message encryption approach:
   - Detail how to preserve end-to-end encryption for stored messages
   - Explain how the recipient's public key would be used to ensure only they can decrypt messages
   - Consider forward secrecy implications and potential mitigations

3. Retrieval protocol:
   - Design a mechanism for clients to retrieve pending messages when coming online
   - Include authentication requirements to prevent unauthorized access
   - Consider bandwidth and storage optimizations

4. Implementation strategy:
   - Suggest modifications to the existing codebase in the messaging and DHT directories
   - Outline new components or interfaces needed
   - Provide pseudocode for key functions

5. Security considerations:
   - Address metadata protection concerns
   - Evaluate timing attack vectors
   - Consider spam/DoS protections for the storage system
   - Explain how to handle message expiration

Your design should maintain Tox's decentralized nature without introducing centralized servers or single points of failure. The solution should integrate with Tox's existing friend system and leverage the current cryptographic primitives while extending them as needed for the asynchronous context. This must not require additional configuration from the user, it must work for any user by default.

Focus on practical implementation details that would work with our current codebase structure.
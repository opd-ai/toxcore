# TOXCORE-GO DEVELOPMENT PLAN

## PROJECT STATUS: Critical Issues Resolved ✅

### COMPLETED (September 1, 2025)
- **✅ Gap #1**: OnFriendMessage callback signature mismatch (CRITICAL) 
- **✅ Gap #2**: AddFriend method signature mismatch (CRITICAL)
- **✅ Gap #3**: GetSavedata Implementation (HIGH PRIORITY)
- **✅ Gap #5**: Self-Management Methods (HIGH PRIORITY)
- **✅ API Documentation**: Updated README.md with correct examples
- **✅ Test Coverage**: Added comprehensive callback and friend management tests
- **✅ State Persistence**: Implemented savedata serialization and restoration
- **✅ Self Information**: Implemented name and status message management
- **✅ Backward Compatibility**: Maintained C binding compatibility

### NEXT PLANNED ITEMS (Priority Order)

#### HIGH PRIORITY
1. **SendFriendMessage Consistency**
   - **Status**: HIGH PRIORITY - API confusion affects usability
   - **Task**: Decide on single consistent API for message sending
   - **Acceptance Criteria**:
     - Single clear method for sending messages
     - Optional message type parameter with sensible default
     - Update documentation and examples
   - **Estimated Effort**: 1-2 hours

#### MEDIUM PRIORITY
2. **Gap #6: SelfGetAddress Nospam Fix**
   - **Status**: Minor priority, cosmetic issue
   - **Task**: Use actual nospam value instead of zero
   - **Acceptance Criteria**:
     - Generate proper ToxID with instance nospam
     - Maintain nospam state across operations
   - **Estimated Effort**: 30 minutes

#### FUTURE ENHANCEMENTS
4. **Noise-IK Migration** (per migrate.md)
   - **Status**: Security enhancement
   - **Task**: Replace custom handshake with Noise Protocol
   - **Estimated Effort**: 1-2 weeks

5. **Async Message Delivery System** (per async.md)
   - **Status**: Feature request
   - **Task**: Design offline message storage and retrieval
   - **Estimated Effort**: 2-3 days

### IMPLEMENTATION GUIDELINES

#### Next Task Selection Criteria
1. Choose tasks that unblock user adoption
2. Prioritize API surface consistency 
3. Focus on core messaging functionality
4. Maintain backward compatibility

#### Quality Standards (Applied to All Tasks)
- Functions under 30 lines with single responsibility
- Explicit error handling (no ignored returns)
- Use net.Addr interface types for network variables
- Self-documenting code with descriptive names
- >80% test coverage for business logic
- Update documentation for all public API changes

#### Library Selection Rules
1. Standard library first
2. Well-maintained libraries (>1000 stars, updated <6 months)
3. No clever patterns - choose boring, maintainable solutions
4. Document WHY decisions were made, not just WHAT

### CURRENT PROJECT HEALTH

#### Strengths
- ✅ Core API now matches documentation
- ✅ Comprehensive test suite (107+ tests passing)
- ✅ Clean modular architecture
- ✅ C binding compatibility maintained
- ✅ Clear error handling patterns

#### Areas for Improvement
- Some API inconsistencies remain (message sending)
- Limited offline message support

### SUCCESS METRICS
- **API Usability**: Users can follow README.md examples without compilation errors ✅
- **Test Coverage**: >80% coverage for all business logic (currently 107 tests passing) ✅  
- **Documentation**: All public APIs documented with examples ✅
- **Performance**: No performance regressions from API fixes ✅
- **Compatibility**: C bindings continue working ✅

### RECOMMENDED NEXT STEP
**Implement SendFriendMessage Consistency** as it's now the highest priority remaining item that improves API usability and reduces user confusion.

---
*Last Updated: September 1, 2025*
*Next Review: After SendFriendMessage API cleanup*

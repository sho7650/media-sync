# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.
refer to @IMPLEMENTATION_ROADMAP.md

## Development Commands

### TDD Workflow

This project follows strict Test-Driven Development. Use these commands for the Red-Green-Refactor cycle:

```bash
make tdd-red        # Write failing tests first
make tdd-green      # Implement minimal code to pass tests
make tdd-refactor   # Improve code quality while tests pass
```

### Testing

```bash
make test           # Run all tests (unit + integration)
make test-unit      # Run unit tests with coverage
make test-integration # Run integration tests
make coverage       # Generate HTML coverage report
```

### Code Quality

```bash
make lint           # Run golangci-lint
make fmt            # Format code with go fmt
make tidy           # Clean up go.mod
make dev-setup      # Complete development setup
```

### Build & Clean

```bash
make build          # Build daemon and CLI binaries
make clean          # Remove build artifacts
```

## Architecture Overview

This is a plugin-based media synchronization platform with a layered architecture:

### Core Components

**Service Interfaces** (`pkg/core/interfaces/`)

- `Service`: Base interface for all services (lifecycle, health, info)
- `InputService`: Data retrieval services (Tumblr, Instagram, etc.)
- `OutputService`: Data publishing services (file system, cloud storage)
- `TransformService`: Data transformation and processing

**Storage Layer** (`internal/storage/`)

- `StorageManager`: Interface for data persistence
- SQLite-based implementation with migrations
- Handles media items, sync states, and queries

**Configuration System** (`internal/config/`)

- `ConfigManager`: YAML configuration with hot reload
- Environment variable substitution
- Service validation and lifecycle management

### Data Flow Architecture

1. **Input Services** retrieve media from external APIs (Tumblr, etc.)
2. **Storage Layer** persists media items and tracks sync state
3. **Configuration System** manages service settings with hot reload
4. **Plugin System** provides extensible service architecture

### Key Data Types

- `MediaItem`: Core media representation with metadata and checksums
- `DataStream`: Streaming data interface with content and metadata
- `SyncState`: Tracks synchronization progress per service
- `ServiceHealth`: Health monitoring and status reporting

## Documentation-First Development

### Plan-Document-Implement Rule

**MANDATORY**: All design changes must follow this exact sequence:

1. **Plan**: Make design decision or re-plan architecture
2. **Document**: Immediately update CLAUDE.md/IMPLEMENTATION_ROADMAP.md
3. **Implement**: Code according to updated documentation

### Documentation Debt Prevention

- **Never implement before documenting**: Documentation updates are not optional
- **Commit documentation changes first**: Use git to track the reasoning
- **Update impact analysis**: Document how changes affect other phases
- **Immediate consistency**: No gaps between planning and documentation

### Examples

**✅ Correct Process**:
```bash
# 1. PLAN: Decide to re-phase 2.3 → 2.2.2.x
# 2. DOCUMENT: Update IMPLEMENTATION_ROADMAP.md immediately
git add IMPLEMENTATION_ROADMAP.md
git commit -m "docs: Re-phase 2.3 Hot Reload as 2.2.2.x cycles"
# 3. IMPLEMENT: Code according to updated plan
```

**❌ Wrong Process**:
```bash
# Implement first, document later - NEVER DO THIS
git commit -m "feat: Implement hot reload"  # Implementation first
# Later: "Oh, I should update the docs..." - TOO LATE
```

This rule prevents architectural drift and ensures team alignment.

## Common Mistakes & Prevention

### Over-Engineering Prevention

**MANDATORY**: Before implementing any component, ask:

1. **Is this actually needed?** Don't implement features "just in case"
2. **What's the simplest solution?** Start with the most basic approach
3. **Can existing code handle this?** Extend rather than create new components
4. **Is this solving a real problem?** Or creating complexity for imaginary issues

### Specific Anti-Patterns to Avoid

**❌ FileWatcher Anti-Pattern (Sept 2025)**:
```go
// 400+ line FileWatcher with ConfigDelta, debouncing, caching
// For simple config file reload requirement
// WRONG: Over-engineered solution for simple problem
```

**✅ Correct Approach**:
```go
// Direct integration with existing PluginManager
// Simple fsnotify watch for config changes
// RIGHT: Minimal solution for actual requirement
```

### YAGNI Enforcement

**"You Aren't Gonna Need It" Violations**:
- ConfigDelta calculation (unused)
- Event debouncing (unnecessary for config reload)
- Complex event handling (single callback sufficient)
- Plugin-specific file filtering (only need .yaml)

### Complexity Red Flags

Stop and reconsider if:
- Component exceeds 100 lines for "minimal" implementation
- Creating interfaces before knowing concrete requirements
- Adding abstraction layers "for future flexibility"
- Implementing features not in current phase requirements

### TDD Misuse Prevention

**TDD ≠ License to Over-Engineer**:
- Write tests for requirements, not imaginary features
- "Red-Green-Refactor" doesn't mean "Red-Green-AddMoreFeatures"
- Minimal implementation means MINIMAL, not "minimal viable product"
- Stop at GREEN phase if requirements are met

### Decision Checkpoints

Before any significant implementation:
1. **Requirement Validation**: What exactly needs to be solved?
2. **Simplicity Check**: What's the most basic solution?
3. **Existing Code Review**: Can current code be extended?
4. **Future-Proofing Audit**: Are we solving tomorrow's problems today?

These mistakes cost development time, create technical debt, and violate YAGNI principles.

## TDD Guidelines

This project uses GitHub Flow with TDD cycles:

1. Create feature branch from main
2. Write failing test (RED phase)
3. Implement minimal code (GREEN phase)
4. Refactor while keeping tests green
5. Create PR and wait for review
6. Merge after approval

### Quality Requirements

- Minimum 80% test coverage (use `make coverage`)
- All linting must pass (`make lint`)
- Tests must use testify framework for assertions
- Use in-memory SQLite for database tests
- Mock external APIs with httptest

### Testing Patterns

- Interface compliance: `var _ InputService = (*Implementation)(nil)`
- Table-driven tests for validation functions
- Mock external dependencies with testify/mock
- Use `t.TempDir()` for temporary test files

## Development Dependencies

- Go 1.25.0
- testify v1.11.1 for testing framework
- sqlite3 for database operations
- golangci-lint for code quality (install separately)

## Branch Strategy

Follow GitHub Flow:

- `feature/component-name` for new features
- `bugfix/issue-description` for bug fixes
- `refactor/component-name` for improvements
- All work happens in feature branches, never on main

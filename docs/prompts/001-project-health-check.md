---
description: 'comprehensive code quality, documentation accuracy, and project maintainability analysis'
execution-mode: parallel
agents:
  - type: go-architect
    focus: 'analyze architecture, design patterns, code structure, maintainability, and Go idioms'
  - type: go-test-expert
    focus: 'evaluate test coverage, test quality, edge cases, and testing best practices'
  - type: qa-expert
    focus: 'verify documentation accuracy, build system health, CI/CD setup, and overall project quality'
---

# Project Health Check

comprehensive analysis of code quality, documentation accuracy, testing completeness, and project maintainability. verifies the project actually works by running tests, build, and linters.

## when to use

use this prompt when you need to:
- verify project is production-ready and maintainable
- ensure documentation matches implementation
- identify code quality issues and technical debt
- validate test coverage and reliability
- check build system and CI/CD health
- get multi-perspective analysis from specialized agents

## context

this prompt is designed for the local-docs-mcp project, a Go-based MCP server with:
- modular architecture (app/scanner, app/server packages)
- comprehensive documentation (README.md, CLAUDE.md)
- test-driven development with testify
- CI/CD via GitHub Actions with golangci-lint
- direct go commands for build/test/lint

## execution plan

the prompt spawns three parallel agents for different perspectives, then synthesizes findings:

### 1. go-architect agent

analyze architectural quality and maintainability:
- review package structure and separation of concerns (scanner vs server)
- evaluate design patterns and Go idioms compliance
- check error handling consistency and safety
- assess API design and type safety
- identify code smells and potential refactoring opportunities
- verify security practices (path traversal prevention, input validation)
- examine caching strategy and performance optimization

focus areas:
- `app/scanner/` - file discovery, frontmatter parsing, caching, safe path resolution
- `app/server/` - MCP protocol implementation, tool registration, business logic
- `app/main.go` - initialization, CLI setup, server lifecycle

### 2. go-test-expert agent

evaluate testing completeness and quality:
- verify test coverage against 80% target (current: scanner 84.3%, server 86.4%, main 40.9%)
- identify coverage gaps, especially in main.go
- review test structure and table-driven test usage
- check edge case coverage and error path testing
- evaluate test fixtures in testdata/
- verify integration test coverage
- assess benchmark tests quality

run verification:
```bash
go test -race ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

focus on packages with low coverage and critical paths.

### 3. qa-expert agent

verify documentation, build system, and overall quality:
- **documentation accuracy**:
  - verify README.md examples match current implementation
  - check CLAUDE.md reflects actual architecture
  - ensure code comments match behavior

- **build system health**:
  - run `go build` and verify success
  - check `golangci-lint run` passes without errors
  - verify all dependencies are current in go.mod

- **CI/CD verification**:
  - review .github/workflows/ci.yml for completeness
  - check goreleaser configuration validity
  - verify all CI checks would pass

- **dependency health**:
  - check go.mod for outdated or vulnerable dependencies
  - verify vendor directory is current (if used)

run verification:
```bash
go test -race ./...
go build
golangci-lint run
```

## synthesis and reporting

after all agents complete, synthesize findings into comprehensive report:

### report structure

1. **executive summary**
   - overall project health score
   - critical issues requiring immediate attention
   - project strengths

2. **code quality findings**
   - architectural insights from go-architect
   - specific issues with file:line references
   - refactoring recommendations

3. **testing assessment**
   - current coverage by package
   - gaps requiring new tests
   - test quality improvements

4. **documentation issues**
   - outdated examples or instructions
   - missing documentation
   - accuracy problems

5. **build and CI/CD**
   - build system issues
   - linter errors (if any)
   - CI/CD recommendations

6. **action items**
   - prioritized list of improvements
   - quick wins vs long-term work
   - recommended next steps

## verification commands

before reporting, verify project actually works:

```bash
# run tests with race detector
go test -race ./...

# build binary
go build

# run linter (must pass for CI)
golangci-lint run

# check coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

report any failures immediately with full error context.

## expected output

comprehensive markdown report with:
- clear sections for each analysis area
- specific file:line references for all issues
- actionable recommendations with priority levels
- verification results (test/build/lint status)
- overall maintainability assessment

prioritize honesty over praise - identify real issues that need addressing.

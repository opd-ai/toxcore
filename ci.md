TASK DESCRIPTION:
Create a comprehensive GitHub Actions workflow file that implements a complete CI/CD pipeline for this repository.

CONTEXT:
This project requires an automated build and testing process using GitHub Actions. The workflow should verify code quality, build the software, and manage releases. The configuration should follow current GitHub Actions best practices, with separate jobs for testing, building, and deployment phases.

INSTRUCTIONS:
1. Create a complete GitHub Actions workflow file (YAML format) that:
   a. Triggers on push events to main/master branch and on pull requests
   b. Triggers on tag creation events matching semantic versioning pattern (v*.*.*)
   c. Uses appropriate GitHub Actions runner (ubuntu-latest or other if needed)

2. Implement a testing job that:
   a. Sets up the required language environment with proper caching
   b. Installs all necessary dependencies
   c. Runs all available test suites with appropriate flags
   d. Reports test results and code coverage

3. Implement a build job that:
   a. Depends on successful test completion
   b. Compiles/builds the application for all supported platforms
   c. Generates build artifacts with meaningful names including version information
   d. Uploads build artifacts to the GitHub Actions run using appropriate action

4. Implement a release job that:
   a. Only runs when the workflow is triggered by a tag matching v*.*.*
   b. Creates a GitHub Release with the tag name as the release name
   c. Generates release notes from commit messages since the previous tag
   d. Uploads all build artifacts to the GitHub Release
   e. Marks the release as either pre-release or production based on tag format

5. Ensure the workflow includes:
   a. Proper error handling and timeout configurations
   b. Appropriate concurrency settings to prevent redundant runs
   c. Caching mechanisms to speed up builds
   d. Detailed logging for troubleshooting

FORMATTING REQUIREMENTS:
- Use valid YAML syntax with proper indentation
- Include descriptive comments explaining key sections
- Place the workflow file in the correct directory (.github/workflows/)
- Name the workflow file appropriately (e.g., ci.yml, build-and-release.yml)
- Use environment variables for reusable values
- Follow GitHub Actions best practices for job organization and step naming

QUALITY CHECKS:
- Verify that all jobs have appropriate dependencies (needs:) configured
- Ensure the workflow handles different operating systems if required
- Confirm that secrets and sensitive data are properly handled
- Check that artifact naming is consistent and includes version information
- Validate that release creation only occurs on proper version tags

EXAMPLES:
Here's a minimal example of how a job structure might look:

```yaml
name: CI/CD Pipeline
on:
  push:
    branches: [main, master]
    tags: ['v*.*.*']
  pull_request:
    branches: [main, master]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      # Additional test steps here

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      # Build steps here

  release:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: build
    runs-on: ubuntu-latest
    steps:
      # Release steps here
````
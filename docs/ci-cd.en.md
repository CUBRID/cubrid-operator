# CI/CD Guide

## Overview

The CI/CD pipeline structure and operation of the cubrid-operator project is explained.

## Overall Structure

- Uses GitHub Actions to automate code quality checks, builds, tests, Docker image builds, and deployments.
- Main workflows are as follows:
  - When PR (Pull Request) is created/updated: Lint, Build automatically executed
  - When main branch is merged: Lint, Build automatically executed

## Workflow Details

### 1. Lint
- Trigger: Every time a commit is made to PR
- Action: Performs code style and static analysis using golangci-lint
- Location: `.github/workflows/lint.yml`
- Detailed settings: Refer to `.golangci.yml` file

### 2. Build
- Trigger: Every time a commit is made to PR
- Action: Builds entire project using go build
- Location: `.github/workflows/build.yml`

## Reference
- Workflow files are located in the `.github/workflows/` directory.
- For detailed settings and examples, refer to each workflow file.

--- 
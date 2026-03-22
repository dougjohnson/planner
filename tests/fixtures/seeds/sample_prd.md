# Sample PRD: Task Manager

A simple task management application for testing document decomposition.

## Overview

This document describes a minimal task manager that allows users to create,
update, and delete tasks. It serves as a test fixture for the fragment system.

## User Stories

- As a user, I can create a new task with a title and description
- As a user, I can mark a task as complete
- As a user, I can delete a task
- As a user, I can view all my tasks

## Technical Requirements

The application should use a local SQLite database for persistence.
All API endpoints should return JSON responses with proper error handling.

## Non-Functional Requirements

- Response time under 200ms for all operations
- Support for concurrent access
- Data integrity enforced via foreign keys

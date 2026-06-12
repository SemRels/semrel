# ADR-004: Core ReleaseNotes Rendering vs. Plugin-specific Formatting

- Status: Accepted
- Date: 2026-06-12

## Context

semrel core provides a canonical release model and changelog generation.
At the same time, provider and hook plugins integrate with target systems
such as GitHub, GitLab, Slack, Teams, and others.

Question: should forge/chat-specific output formatting live in semrel core,
or in the corresponding plugins?

## Decision

Keep rendering in semrel core only for neutral, reusable formats.

Core formats include:
- generic Markdown changelog
- Keep a Changelog variant
- package/ecosystem formats that are not tied to one forge or one chat product

Plugin-specific formatting belongs in plugins, including:
- GitHub/GitLab release body shaping
- Slack/Teams/Matrix/Discord message formatting
- any provider/hook payload shaping that depends on platform conventions

Core passes canonical data (version, changelog, metadata) to plugins.
Plugins adapt that data to their platform-specific payloads.

## Consequences

Positive:
- clear ownership boundary
- less platform coupling in semrel core
- faster plugin iteration without core releases

Negative:
- formatting logic can be duplicated across plugins
- plugin maintainers must keep conventions aligned

## Notes

This ADR aligns with the existing repository split decision in ADR-002.

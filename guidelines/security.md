# Security review rules

- Flag hardcoded secrets, API keys, private keys, and connection strings in diffs.
- Reject SQL built from string concatenation or unparameterized template literals.
- Call out missing authorization checks on new HTTP routes.
- Warn on `dangerouslySetInnerHTML`, `innerHTML`, or unsanitized Markdown rendered as HTML.
- Flag shell execution (`exec`, `spawn`) that interpolates user input.
- Prefer allow-lists over regex denylists for path traversal and redirect URLs.
- New dependencies must be justified; note packages with known supply-chain risk if the diff adds them.
- Do not request exploit proof-of-concepts. Report impact, location, and a safe fix.

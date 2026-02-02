# Instructions for agents

- Use `make help` to find available development targets
- Use the latest Go stable release when working on Go code
- Use the latest Node.js LTS release with pnpm when working on TypeScript code
- Before running Go tests, run `make deps-backend` to install dependencies
- Before running TypeScript tests, run `make deps-frontend` to install dependencies
- Before committing Go changes, run `make fmt` to format, and run `make lint-go` to lint
- Before committing TypeScript changes, run `make lint-js` to lint
- Before committing `go.mod` changes, run `make tidy`
- Before committing new Go files, add the current year into the copyright header
- Before committing any files, remove all trailing whitespace from source code lines

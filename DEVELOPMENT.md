# Developing the Plugin

Following prerequisites must be installed for developing the plugin

- [`yarn`](https://yarnpkg.com/)
- [`mage`](https://magefile.org/)


## Frontend

1. Install dependencies

   ```bash
   yarn install
   ```

2. Build plugin in development mode or run in watch mode

   ```bash
   yarn dev

   # or

   yarn watch
   ```

3. Build plugin in production mode

   ```bash
   yarn build
   ```

4. Run the tests (using Jest)

   ```bash
   # Runs the tests and watches for changes
   yarn test
   
   # Exists after running all the tests
   yarn lint:ci
   ```

5. Spin up a Grafana instance and run the plugin inside it (using Docker)

   ```bash
   yarn server
   ```

6. Run the E2E tests (using Cypress)

   ```bash
   # Spin up a Grafana instance first that we tests against 
   yarn server
   
   # Start the tests
   yarn e2e
   ```

7. Run the linter

   ```bash
   yarn lint
   
   # or

   yarn lint:fix
   ```

## Backend

1. Update [Grafana plugin SDK for Go](https://grafana.com/developers/plugin-tools/introduction/grafana-plugin-sdk-for-go) dependency to the latest minor version:

   ```bash
   go get -u github.com/grafana/grafana-plugin-sdk-go
   go mod tidy
   ```

2. Build backend plugin binaries for Linux, Windows and Darwin:

   ```bash
   mage -v
   ```

3. List all available Mage targets for more commands:

   ```bash
   mage -l
   ```

4. Navigate to the [Locally Running Grafana](http://localhost:3000).

### Debugging

in `docker-compose.yaml` file, search for `development: false` and replace it with `development: true`.

This starts an enhanced framework that automatically rebuilds and reloads the plugin inside the grafana container.
The go debugger is attached to the plugin and listen on localhost:2345. 

You can start a debug session from your local IDE and set breaking points as you would do on local development.

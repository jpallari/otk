# gitsync

Synchronise Git repositories.

## Installation

First, you need to install [Go compiler](https://golang.org/dl/) version 1.24 or higher.
Make sure `~/go/bin` or `$GOPATH/bin` is in your `PATH` variable.

After that, you can install gitsync with the Go tools:

```shell
go install go.lepovirta.org/otk/cmd/otk-gitsync
```

You can then execute gitsync with `otk-gitsync`. Example:

```shell
otk-gitsync -h
```

## Usage

The following command line flags are accepted:

- `-config`:
  Path to a configuration file.
  Use '-' to read from STDIN.
  By default, config is read from STDIN. (default "-")
- `-credentials`:
  Path to a credentials file.
  Use '-' to read from STDIN.
- `-once`:`
  Run Git sync only once instead of the repeatedly as specified in the configuration.
- `-run`:
  Run the Git sync.
  If not enabled, a dry run will be executed instead.

## Configuration

The configuration and credentials files use JSON format.
See the details below on configuration formats.

### Simple configuration

If you only need to synchronise a single Git repository to one or more targets, you can use the simple configuration format.
Here is what a simple configuration looks like.
The values in the configuration below are default values.

```jsonc
{
    // Path to a Git repository in the file system that
    // is to be synchronised to remote repositories. By default,
    // the current working directory is used.
    "path": "",

    // Targets contain the information on which Git repositories to
    // synchronise the local Git repository to. The target key is
    // used as the remote identifier during mirroring and in logs.
    "targets": {
        "<name>": {
            // The remote URL where the Git repository is located.
            // E.g. https://github.com/jpallari/otk.git
            "url": "",

            // When the flag is set to `true`, the Git repository is downloaded
            // to memory rather than the file system.
            "inMemory": false,

            // Specifies the path where the Git repository is downloaded to.
            // When `inMemory` is set to `true`, this value is ignored.
            // When left unset, a temporary directory is created for the Git repository.
            "localPath": "",

            // Specifies which authentication method is used when connecting to the Git repository.
            // When set, gitsync verifies that credentials are found for the repository from
            // either this configuration or the credentials configuration.
            // Leaving the value unset allows the authentication to be auto-detected.
            // Possible values: none, http-token, http, ssh-agent, ssh.
            "authMethod": "",

            // Use a HTTP token used for connecting to HTTPS-based Git repositories.
            "httpToken": "",

            // Use HTTP basic auth credentials used for connecting to HTTPS-based Git repositories
            "httpCredentials": {
                "username": "",
                "password": ""
            },

            // Use SSH credentials for connecting to SSH-based Git repositories
            "sshCredentials": {
                // When the flag is set to `true`, SSH agent is used for acquiring
                // the SSH key for connecting to the remote repository.
                "useAgent": false,

                // The SSH username to use for connecting to the Git repository.
                "username": "git",

                // Path to a SSH key used for connecting to the Git repository.
                "keyPath": "",

                // The password for unlocking the SSH key specified in the key path.
                "keyPassword": "",

                // The SSH host key expected from the remote server.
                // When left unset, host key is checked from the known hosts file.
                // The host key is supplied in authorized_keys format according to sshd(8) manual page.
                "hostKey": "",

                // File paths where known SSH hosts are recorded.
                // When left unset, the default hosts paths are used (e.g. ~/.ssh/known_hosts).
                // The files in the given paths must be in ssh_known_hosts format according to
                // sshd(8) manual page.
                "knownHostsPaths": [],

                // When the flag is set to `true`, the SSH host key for the Git repository
                // is not verified.
                // WARNING! Not recommended to be used in production!
                "ignoreHostKey": false,
            },

            // How frequently to synchronise the Git repository.
            // Specified as a sequence of decimal numbers, each with optional fraction
            // and a unit suffix, such as "300ms", "-1.5h" or "2h45m".
            // Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
            "interval": "1h",

            // List of branches to synchronise to the target Git repository.
            // Can be specified as a regex when surrounding the string with `/` characters
            // e.g. `/main.*/`
            "branches": [],

            // List of tags to synchronise to the target Git repository.
            // Can be specified as a regex when surrounding the string with `/` characters
            // e.g. `/v[0-9]+/`
            "tags": []
        }
    }
}
```

### Standard configuration

If you need to synchronise a multiple Git repositories to one or more targets, you can use the standard configuration format.
Here is what a standard configuration looks like.
The values in the configuration below are default values.

```jsonc
{
    // Details of all of the Git repositories involved during synchronisation.
    // The key is the ID of the repository, which is referenced in the mappings
    // source and target fields.
    "repositories": {
        "<id>": {
            // The remote URL where the Git repository is located.
            // E.g. https://github.com/jpallari/otk.git
            "url": "",

            // When the flag is set to `true`, the Git repository is downloaded
            // to memory rather than the file system.
            "inMemory": false,

            // Specifies the path where the Git repository is downloaded to.
            // When `inMemory` is set to `true`, this value is ignored.
            // When left unset, a temporary directory is created for the Git repository.
            "localPath": "",

            // Specifies which authentication method is used when connecting to the Git repository.
            // When set, gitsync verifies that credentials are found for the repository from
            // either this configuration or the credentials configuration.
            // Leaving the value unset allows the authentication to be auto-detected.
            // Possible values: none, http-token, http, ssh-agent, ssh.
            "authMethod": "",

            // Use a HTTP token used for connecting to HTTPS-based Git repositories.
            "httpToken": "",

            // Use HTTP basic auth credentials used for connecting to HTTPS-based Git repositories
            "httpCredentials": {
                "username": "",
                "password": ""
            },

            // Use SSH credentials for connecting to SSH-based Git repositories
            "sshCredentials": {
                // When the flag is set to `true`, SSH agent is used for acquiring
                // the SSH key for connecting to the remote repository.
                "useAgent": false,

                // The SSH username to use for connecting to the Git repository.
                "username": "git",

                // Path to a SSH key used for connecting to the Git repository.
                "keyPath": "",

                // The password for unlocking the SSH key specified in the key path.
                "keyPassword": "",

                // The SSH host key expected from the remote server.
                // When left unset, host key is checked from the known hosts file.
                // The host key is supplied in authorized_keys format according to sshd(8) manual page.
                "hostKey": "",

                // File paths where known SSH hosts are recorded.
                // When left unset, the default hosts paths are used (e.g. ~/.ssh/known_hosts).
                // The files in the given paths must be in ssh_known_hosts format according to
                // sshd(8) manual page.
                "knownHostsPaths": [],

                // When the flag is set to `true`, the SSH host key for the Git repository
                // is not verified.
                // WARNING! Not recommended to be used in production!
                "ignoreHostKey": false,
            }
        }
    },

    // Mappings specifies which Git repositories are synchronised where.
    "mappings": [
        {
            // The ID of the repository to sync to the targets.
            "source": "",

            // The IDs of the repositories to sync the source repository to.
            "targets": []

            // How frequently to synchronise the Git repository.
            // Specified as a sequence of decimal numbers, each with optional fraction
            // and a unit suffix, such as "300ms", "-1.5h" or "2h45m".
            // Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
            "interval": "1h",

            // List of branches to synchronise to the target Git repository.
            // Can be specified as a regex when surrounding the string with `/` characters
            // e.g. `/main.*/`
            "branches": [],

            // List of tags to synchronise to the target Git repository.
            // Can be specified as a regex when surrounding the string with `/` characters
            // e.g. `/v[0-9]+/`
            "tags": []
        }
    ]
}
```


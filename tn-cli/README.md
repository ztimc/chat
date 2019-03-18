# Command Line Client for Tinode

This is a command line chat client. It's written in Python as a demonstration of Tinode [gRPC](https://grpc.io) [API](../pbx/).

Python 2.7 or 3.4+ is required. PIP 9.0.1 or newer is required.

Install dependencies:
```
$ python -m pip install -r requirements.txt
```

Run the client from the command line:
```
python tn-cli.py --login-basic=alice:alice123
```

If you are updating an existent installation, make sure the `tinode_grpc` version matches the [server](../server/) version. Upgrade `tinode_grpc` if needed:
```
python -m pip install --upgrade tinode_grpc==X.XX.XX
```
where `X.XX.XX` is the version number which must match the server version number.

The client takes optional parameters:

 * `--host` is the address of the gRPC server to connect to; default `localhost:6061`.
 * `--web-host` is the address of Tinode web server, used for file uploads only; default `localhost:6060`.
 * `--ssl` the server requires a secure connection (SSL)
 * `--ssl-host` the domain name to use for SNI if different from the `--host` domain name.
 * `--login-basic` is the `login:password` to be authenticated with.
 * `--login-token` is the token to be authenticated with.
 * `--login-cookie` direct the client to read the token from the cookie file `.tn-cli-cookie` generated during an earlier login.
 * `--no-login` do not login even if cookie file is present.
 * `--api-key` web API key for file uploads; default `AQEAAAABAAD_rAp4DJh05a1HAwFT3A6K`

If multiple `login-XYZ` are provided, `login-cookie` is considered first, then `login-token` then `login-basic`. Authentication with token (and cookie) is much faster than with the username-password pair.

## Commands

### Local

* `.await` - issue a gRPC call and wait for completion, optionally assign result to a variable.
* `.exit` - terminate execution and exit the CLI; also `.quit`.
* `.log` - write a value of a variable to `stdout`.
* `.must` - issue a gRPC call and wait for completion, optionally assign result to a variable; raise an exception if result is not a success.
* `.quit` - terminate execution and exit the CLI; also `.exit`.
* `.sleep` - suspend the process for a number of milliseconds.
* `.use` - set default user (on_behalf_of user) or topic

### gRPC calls

* `acc` - create  or modify an account
* `login` - authenticate current session
* `sub` - subscribe to topic
* `leave` - detach or unsubscribe from topic
* `pub` - post message to topic
* `get` - query topic for metadata or messages
* `set` - update topic metadata
* `del` - delete message(s), topic, subscription, or user
* `note` - send notification

### HTTP requests

* `upload` - upload file out of band


Type `<command> -h` for help

See some of these commands in use in the [sample-script.txt](sample-script.txt). Try it as
```
python tn-cli.py --no-login < sample-script.txt
```

## Connecting to secure (HTTPS) server

If the server is configured to use TLS, i.e. running as `httpS://my-server.example.com/`, the gRPC endpoint also uses the same SSL certificate. In that case add the `--ssl` option.

If you want to connect to the secure gRPC endpoint over a local network or under a different name i.e. as `localhost` instead of  `my-server.example.com` in this example, you must specify the SSL domain name to use, otherwise the server will not be able to find the right SSL certificate:
```
python tn-cli.py --host=localhost:6001 --ssl --ssl-host=my-server.example.com
```
The `--ssl-host` option makes the connection susceptible to the [Man-in-the-middle attack](https://en.wikipedia.org/wiki/Man-in-the-middle_attack), so don't do it over public networks.

## Crash on shutdown

Python 3.6 sometimes crashes on shutdown with a message `Fatal Python error: PyImport_GetModuleDict: no module dictionary!`. That happens because Python is buggy: https://bugs.python.org/issue26153

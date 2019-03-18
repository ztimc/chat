# Installing Tinode

## Installing from Binaries

1. Visit the [Releases page](https://github.com/tinode/chat/releases/), choose the latest or otherwise the most suitable release. From the list of binaries download the one for your database and platform. Once the binary is downloaded, unpack it to a directory of your choosing, `cd` to that directory.

2. Make sure your database is running. Make sure it's configured to accept connections from `localhost`. In case of MySQL, Tinode will try to connect as `root` without the password. See notes below (_Building from Source_, section 4) on how to configure Tinode to use a different user or a password. MySQL 5.7 or above is required. MySQL 5.6 or below **will not work**.

3. Run the database initializer `init-db` (or `init-db.exe` on Windows):
	```
	./init-db -data=data.json
	```

4. Run the `tinode` (or `tinode.exe` on Windows) server. It will work without any parameters.
	```
	./tinode
	```

5. Test your installation by pointing your browser to http://localhost:6060/


## Docker

See [instructions](./docker/README.md)


## Building from Source

1. Install [Go environment](https://golang.org/doc/install). Make sure Go version is at least 1.9. Building with Go 1.8 or below **will fail**!

2. Make sure either [RethinkDB](https://www.rethinkdb.com/docs/install/) or MySQL (or MariaDB or Percona) is installed and running. MySQL 5.7 or above is required. MySQL 5.6 or below **will not work**.

3. Fetch, build Tinode server and tinode-db database initializer:
 - **RethinkDb**:
	```
	go get -tags rethinkdb github.com/tinode/chat/server && go install -tags rethinkdb github.com/tinode/chat/server
	go get -tags rethinkdb github.com/tinode/chat/tinode-db && go install -tags rethinkdb github.com/tinode/chat/tinode-db
	```
 - **MySQL**:
	```
	go get -tags mysql github.com/tinode/chat/server && go install -tags mysql github.com/tinode/chat/server
	go get -tags mysql github.com/tinode/chat/tinode-db && go install -tags mysql github.com/tinode/chat/tinode-db
	```

	Note the required **`-tags rethinkdb`** or **`-tags mysql`** build option.

	You may also optionally define `main.buildstamp` for the server by adding a build option, for instance, with a timestamp:
	```
	-ldflags "-X main.buildstamp=`date -u '+%Y%m%dT%H:%M:%SZ'`"
	```
	The value of `buildstamp` will be sent by the server to the clients.


4. Open `tinode.conf`. Check that the database connection parameters are correct for your database. If you are using MySQL make sure [DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name) in `"mysql"` section is approprite for your MySQL installation. Option `parseTime=true` is required.
```js
	"mysql": {
		"dsn": "root@tcp(localhost)/tinode?parseTime=true",
		"database": "tinode"
	},
```

5. Download javascript client for testing:
 - https://github.com/tinode/webapp/archive/master.zip
 - https://github.com/tinode/tinode-js/archive/master.zip

6. Now that you have built the binaries, follow instructions in the _Installing from Binaries_ section for running the binaries except in step 3 the initializer is called `tinode-db` (`tinode-db.exe` on Windows), not `init-db`.

## Running a Standalone Server

1. Make sure your database is running:
 - **RethinkDB**: https://www.rethinkdb.com/docs/start-a-server/
	```
	rethinkdb --bind all --daemon
	```
 - **MySQL**: https://dev.mysql.com/doc/mysql-startstop-excerpt/5.7/en/programs-server.html
	```
	mysql.server start
	```

2. Run DB initializer
	```
	$GOPATH/bin/tinode-db -config=$GOPATH/src/github.com/tinode/chat/tinode-db/tinode.conf
	```
	add `-data=$GOPATH/src/github.com/tinode/chat/tinode-db/data.json` flag if you want sample data to be loaded:
	```
	$GOPATH/bin/tinode-db -config=$GOPATH/src/github.com/tinode/chat/tinode-db/tinode.conf -data=$GOPATH/src/github.com/tinode/chat/tinode-db/data.json
	```

	DB intializer needs to be run only once per installation. See [instructions](tinode-db/README.md) for more options.

3. Unpack JS client to a directory, for instance `$HOME/tinode/example-react-js/` by first unzipping `https://github.com/tinode/webapp/archive/master.zip` then extract `tinode.js` from `https://github.com/tinode/tinode-js/archive/master.zip` to the same directory.

4. Run server
	```
	$GOPATH/bin/server -config=$GOPATH/src/github.com/tinode/chat/server/tinode.conf -static_data=$HOME/tinode/example-react-js/
	```

5. Test your installation by pointing your browser to [http://localhost:6060/](http://localhost:6060/). The static files from the `-static_data` path are served at web root `/`. You can change this by editing the line `static_mount` in the config file.

6.  If you want to use the [Android client](https://github.com/tinode/tindroid) and want push notification to work, find the section `"push"` in `tinode.conf`, item `"name": "fcm"`, then change `"enabled"` to `true`. Go to [https://console.firebase.google.com/](https://console.firebase.google.com/) (https://console.firebase.google.com/project/**NAME-OF-YOUR-PROJECT**/settings/cloudmessaging) and get a server key. Paste the key to the `"api_key"` field. See more at https://github.com/tinode/tindroid.

## Running a Cluster

- Install RethinkDB, run it stanalone or in [cluster mode](https://www.rethinkdb.com/docs/start-a-server/#a-rethinkdb-cluster-using-multiple-machines). Run DB initializer, unpack JS files as described in the previous section.

- Cluster expects at least two nodes. A minimum of three nodes is recommended.

- The following section configures the cluster.

```
	"cluster_config": {
		// Name of the current node.
		"self": "",
		// List of all cluster nodes, including the current one.
		"nodes": [
			{"name": "one", "addr":"localhost:12001"},
			{"name": "two", "addr":"localhost:12002"},
			{"name": "three", "addr":"localhost:12003"}
		],
		// Configuration of failover feature. Don't change.
		"failover": {
			"enabled": true,
			"heartbeat": 100,
			"vote_after": 8,
			"node_fail_after": 16
		}
	}
```
* `self` is the name of the current node. Generally it's more convenient to specify the name of the current node at the command line using `cluster_self` option. Command line value overrides the config file value. If the value is not provided either in the config file or through the command line, the clustering is disabled.
* `nodes` defines individual cluster nodes. The sample defines three nodes named `one`, `two`, and `tree` running at the localhost at the specified cluster communication ports. Cluster addresses don't need to be exposed to the outside world.
* `failover` is an experimental feature which migrates topics from failed cluster nodes keeping them accessible:
  * `enabled` turns on failover mode; failover mode requires at least three nodes in the cluster.
  * `heartbeat` interval in milliseconds between heartbeats sent by the leader node to follower nodes to ensure they are accessible.
  * `vote_after` number of failed heartbeats before a new leader node is elected.
  * `node_fail_after` number of heartbeats that a follower node misses before it's cosidered to be down.

If you are testing the cluster with all nodes running on the same host, you also must override the `listen` port. Here is an example for launching two cluster nodes from the same host using the same config file:
```
./server -config=./tinode.conf -static_data=./example-react-js/ -listen=:6060 -cluster_self=one &
./server -config=./tinode.conf -static_data=./example-react-js/ -listen=:6061 -cluster_self=two &
```

### Note on Running the Server in Background

There is [no clean way](https://github.com/golang/go/issues/227) to daemonize a Go process internally. One must use external tools such as shell `&` operator, `systemd`, `launchd`, `SMF`, `daemon tools`, `runit`, etc. to run the process in the background.

Specific note for [nohup](https://en.wikipedia.org/wiki/Nohup) users: an `exit` must be issued immediately after `nohup` call to close the foreground session cleanly:

```
nohup $GOPATH/bin/server -config=$GOPATH/src/github.com/tinode/chat/server/tinode.conf -static_data=$HOME/tinode/example-react-js/ &
exit
```

Otherwise `SIGHUP` may be received by the server if the shell connection is broken before the ssh session has terminated (indicated by `Connection to XXX.XXX.XXX.XXX port 22: Broken pipe`). In such a case the server will shutdown because `SIGHUP` is intercepted by the server and interpreted as a shutdown request.

For more details see https://github.com/tinode/chat/issues/25.

Presentator v2 to v3 migration tool
======================================================================

CLI that takes care for migrating the database and uploaded files from Presentator v2 to v3.

With that said, there are some notable and breaking changes in Presentator v3:

- If there are multiple OAuth2 accounts from the **same** provider linked to a single Presentator user (eg. 2 Google accounts associated to 1 Presentator user) the migration will keep only the last one linked.

- The "Project Guidelines" are deprecated and no longer available.

- For superuser access you could use the PocketBase Dashboard located at `https://yourPresentatorApp.com/_/`.


## Setup

0. ⚠️ The migration tool is expected to be executed locally while having access to an existing Presentator v2 installation (_at minimum the DB server must be running_) to COPY the data from.

1. Before starting the migration tool, you'll need to have a configured local Presentator v3.

    Presentator v3 is built on top on [PocketBase](https://pocketbase.io) and it is distributed as a single "all-in-one" portable file.

    [Go to the Presentator v3 releases page](https://github.com/presentator/presentator/releases) and download the executable for your platform.

    Navigate to the extracted directory and start the executable with `./presentator serve`.

    Go to your browser and open `http://127.0.0.1:8090/_/` to configure the appropriate app settings from the PocketBase Dashboard > Settings, including the S3 file storage credentials if you are planning to use one.

    Once done, you can stop the process and will notice that it has created a `pb_data` directory next to the executable. This is where your Presentator v3 app data lives and when deploying on production it will be enough to just upload only the executable and the `pb_data` directory, but more on that later.

3. Create a migration `config.json` file and place it next to your `pb_data` (_remove the comments_):

    - if your old Presentator v2 files are stored locally:
    ```js
    {
        "v3DataDir":      "./pb_data", // path to the pb_data dir
        "v2DBDriver":     "mysql", // "pgx" for PostgreSQL
        "v2DBConnection": "username:password@localhost/presentator", // must be a valid DSN
        "v2LocalStorage": "/path/to/your/old/presentator/web/storage"
    }
    ```

    - if your old Presentator v2 files are stored in S3:
    ```js
    {
        "v3DataDir":      "./pb_data", // path to the pb_data dir
        "v2DBDriver":     "mysql", // "pgx" for PostgreSQL
        "v2DBConnection": "username:password@address/dbname", // must be a valid DSN
        "v2S3Storage": {
            "endpoint":       "",
            "bucket":         "",
            "region":         "",
            "accessKey":      "",
            "secret":         "",
            "forcePathStyle": false
        }
    }
    ```

4. [Download the migration tool for your platform](https://github.com/presentator/v2tov3migrate/releases) and for example place it next to your `pb_data`.

5. Start the migration tool with `./v2tov3migrate` and wait for the process to finish (_it could take some time to complete_).

6. Verify that the content was migrated properly by starting the Presentator v3 executable - `./presentator serve` and navigating to `http://127.0.0.1:8090`.

   Once you've confirmed that everything is OK, you can remove the old Presentator v2 data, the `v2tov3migrate` and `config.json` files, and you should be ready to deploy your new Presentator v3 installation.
   For more information on this, please refer to [Presentator v3 - Going to production](https://github.com/presentator/presentator#going-to-production).

> [!TIP]
> The migration tool is "incremental" and it could be run multiple times.
> It will attempt to sync new, changed or deleted records.
>
> This also means that in case of an error (eg. lack of disk space), next time when you start it again it should be able to continue from where it left.

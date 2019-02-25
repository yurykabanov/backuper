module github.com/yurykabanov/backuper

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.13.1
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang-migrate/migrate/v4 v4.2.4
	github.com/jmoiron/sqlx v1.2.0
	github.com/mattn/go-sqlite3 v1.9.0
	github.com/pkg/errors v0.8.1
	github.com/robfig/cron v0.0.0-20180505203441-b41be1df6967
	github.com/sirupsen/logrus v1.3.0
	github.com/spf13/pflag v1.0.3
	github.com/spf13/viper v1.3.1
	github.com/stretchr/testify v1.3.0
	golang.org/x/net v0.0.0-20190125091013-d26f9f9a57f3 // indirect
	golang.org/x/sys v0.0.0-20190204203706-41f3e6584952 // indirect
)

replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.3.0

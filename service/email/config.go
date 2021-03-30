package email

type Config struct {
	// 邮箱域名 smtp.xx.com
	Host string `toml:"email_host"`
	// 通信端口
	Port string `toml:"port"`
	// 邮箱地址 xxxxxxxx@xx.com
	User string `toml:"email_user"`
	// 邮箱授权码（并不一定是邮箱密码） xxxxxxxxxxxxxxxx
	Pwd string `toml:"email_pwd"`
}

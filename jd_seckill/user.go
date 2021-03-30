package jd_seckill

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	
	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	
	"JDSeckill/conf"
	"JDSeckill/utils"
)

type User struct {
	client *resty.Client
	conf   *conf.Config
	log    *zap.Logger
}

func NewUser(client *resty.Client, conf *conf.Config, log *zap.Logger) *User {
	return &User{client: client.SetDebug(conf.Debug), conf: conf, log: log}
}

func (u *User) loginPage() {
	req := u.client.NewRequest()
	req.SetHeader("User-Agent", u.conf.UserAgent)
	req.SetHeader("Connection", "keep-alive")
	req.SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3")
	_, _ = req.Get("https://passport.jd.com/new/login.aspx")
}

func (u *User) QrLogin() (string, error) {
	// 登录页面
	u.loginPage()
	// 二维码登录
	client := u.client
	client.SetHeader("User-Agent", u.conf.UserAgent)
	client.SetHeader("Referer", "https://passport.jd.com/new/login.aspx")
	resp, err := client.R().SetOutput("static/qr_code.png").Get("https://qr.m.jd.com/show?appid=133&size=300&t=" + strconv.Itoa(int(time.Now().Unix()*1000)))
	if err != nil || resp.StatusCode() != http.StatusOK {
		u.log.Error("获取二维码失败", zap.Error(err))
		return "", errors.New("获取二维码失败")
	}

	cookies := resp.Cookies()
	wlfstkSmdl := ""
	for _, cookie := range cookies {
		if cookie.Name == "wlfstk_smdl" {
			wlfstkSmdl = cookie.Value
			break
		}
	}
	client.SetCookies(cookies)
	u.log.Info("二维码获取成功，请打开京东APP扫描")
	utils.OpenImage("static/qr_code.png")
	return wlfstkSmdl, nil
}

func (u *User) qrcodeTicket(wlfstkSmdl string) (string, error) {
	client := u.client
	client.SetHeader("User-Agent", u.conf.UserAgent)
	client.SetHeader("Referer", "https://passport.jd.com/new/login.aspx")
	appid := 133
	callback := "jQuery" + strconv.Itoa(utils.Rand(1000000, 9999999))
	token := wlfstkSmdl
	other := strconv.Itoa(int(time.Now().Unix()*1000))
	resp, err := client.R().Get(fmt.Sprintf("https://qr.m.jd.com/check?callback=%s&appid=%d&token=%s&_=%s", callback, appid,  token, other))
	if err != nil || resp.StatusCode() != http.StatusOK {
		u.log.Error("获取二维码扫描结果异常", zap.Error(err))
		return "", errors.New("获取二维码扫描结果异常")
	}
	body := string(resp.Body())
	if gjson.Get(body, "code").Int() != 200 {
		u.log.Info(gjson.Get(body, "msg").String())
		return "", nil
	}
	u.log.Info("已完成手机客户端确认")
	return gjson.Get(body, "ticket").String(), nil
}

func (u *User) QrcodeTicket(wlfstkSmdl string) (string, error) {
	var ticket string
	var err error
	for {
		ticket, err = u.qrcodeTicket(wlfstkSmdl)
		if err == nil && ticket != "" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return ticket, nil
}


func (u *User) TicketInfo(ticket string) (string, error) {
	client := u.client
	client.SetHeader("User-Agent", u.conf.UserAgent)
	client.SetHeader("Referer", "https://passport.jd.com/uc/login?ltype=logout")
	resp, err := client.R().Get("https://passport.jd.com/uc/qrCodeTicketValidation?t=" + ticket)
	if err != nil || resp.StatusCode() != http.StatusOK {
		u.log.Error("二维码信息校验失败", zap.Error(err))
		return "", errors.New("二维码信息校验失败")
	}
	
	body := string(resp.Body())

	if gjson.Get(body, "returnCode").Int() == 0 {
		u.log.Info("二维码信息校验成功")
		return "", nil
	} else {
		u.log.Error("二维码信息校验失败")
		return "", errors.New("二维码信息校验失败")
	}
}

func (u *User) RefreshStatus() error {
	client := u.client
	client.SetHeader("User-Agent", u.conf.UserAgent)
	resp, err := client.R().Get("https://order.jd.com/center/list.action?rid=" + strconv.Itoa(int(time.Now().Unix()*1000)))
	if err == nil && resp.StatusCode() == http.StatusOK {
		return nil
	} else {
		u.log.Error("登录失效", zap.Error(err))
		return errors.New("登录失效")
	}
}

func (u *User) GetUserInfo() (string, error) {
	client := u.client.SetDebug(u.conf.Debug)
	client.SetHeader("User-Agent", u.conf.UserAgent)
	client.SetHeader("Referer", "https://order.jd.com/center/list.action")
	resp, err := client.R().Get("https://passport.jd.com/user/petName/getUserInfoForMiniJd.action?callback=" + strconv.Itoa(utils.Rand(1000000, 9999999)) + "&_=" + strconv.Itoa(int(time.Now().Unix()*1000)))
	if err != nil || resp.StatusCode() != http.StatusOK {
		u.log.Error("获取用户信息失败", zap.Error(err))
		return "", errors.New("获取用户信息失败")
	} else {
		
		body := string(resp.Body())
		b, _ := utils.GbkToUtf8([]byte(gjson.Get(body, "nickName").String()))
		return string(b), nil
	}
}

package jd_seckill

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	"JDSeckill/conf"
	email2 "JDSeckill/service/email"
	"JDSeckill/utils"
)

type Seckill struct {
	client *resty.Client
	conf   *conf.Config
	log    *zap.Logger
}

func NewSeckill(client *resty.Client, conf *conf.Config, logger *zap.Logger) *Seckill {
	return &Seckill{client: client, conf: conf, log: logger}
}

func (s *Seckill) SkuTitle() (string, error) {
	skuId := s.conf.SkuId
	req := s.client
	resp, err := req.GetClient().Get(fmt.Sprintf("https://item.jd.com/%s.html", skuId))
	if err != nil || resp.StatusCode != http.StatusOK {
		s.log.Error("访问商品详情失败", zap.Error(err))
		return "", errors.New("访问商品详情失败")
	}
	doc, _ := goquery.NewDocumentFromReader(resp.Body)
	return strings.TrimSpace(doc.Find(".sku-name").Text()), nil
}

func (s *Seckill) MakeReserve() {
	shopTitle, err := s.SkuTitle()
	if err != nil {
		s.log.Error("获取商品信息失败", zap.Error(err))
	} else {
		s.log.Info("商品名称", zap.String("", shopTitle))
	}
	skuId := s.conf.SkuId
	req := s.client
	req.SetHeader("User-Agent", s.conf.UserAgent)
	req.SetHeader("Referer", fmt.Sprintf("https://item.jd.com/%s.html", skuId))
	resp, err := req.R().Get("https://yushou.jd.com/youshouinfo.action?callback=fetchJSON&sku=" + skuId + "&_=" + strconv.Itoa(int(time.Now().Unix()*1000)))
	if err != nil || resp.StatusCode() != http.StatusOK {
		s.log.Error("预约商品失败", zap.Error(err))
	} else {
		body := resp.String()
		reserveUrl := gjson.Get(body, "url").String()
		req = s.client
		_, _ = req.GetClient().Get("https:" + reserveUrl)
		s.log.Info("预约成功，已获得抢购资格 / 您已成功预约过了，无需重复预约")
	}
}

func (s *Seckill) getSeckillUrl() (string, error) {
	skuId := s.conf.SkuId
	req := s.client
	req.SetHeader("User-Agent", s.conf.UserAgent)
	req.SetHeader("Host", "itemko.jd.com")
	req.SetHeader("Referer", fmt.Sprintf("https://item.jd.com/%s.html", skuId))
	resp, err := req.GetClient().Get("https://itemko.jd.com/itemShowBtn?callback=jQuery{}" + strconv.Itoa(utils.Rand(1000000, 9999999)) + "&skuId=" + skuId + "&from=pc&_=" + strconv.Itoa(int(time.Now().Unix()*1000)))
	if err != nil || resp.StatusCode != http.StatusOK {
		s.log.Error("抢购链接获取失败，稍后自动重试", zap.Error(err))
		return "", errors.New("抢购链接获取失败，稍后自动重试")
	}

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		s.log.Error("写入buffer错误", zap.Error(err))
		return "", errors.New("读响应体错误")
	}
	body := buf.String()

	url := gjson.Get(body, "url").String()
	if url == "" {
		s.log.Info("抢购链接获取失败，稍后自动重试")
		return "", errors.New("抢购链接获取失败，稍后自动重试")
	}
	// https://divide.jd.com/user_routing?skuId=8654289&sn=c3f4ececd8461f0e4d7267e96a91e0e0&from=pc
	url = strings.ReplaceAll(url, "divide", "marathon")
	// https://marathon.jd.com/captcha.html?skuId=8654289&sn=c3f4ececd8461f0e4d7267e96a91e0e0&from=pc
	url = strings.ReplaceAll(url, "user_routing", "captcha.html")
	return url, nil
}

func (s *Seckill) RequestSeckillUrl() {
	user := NewUser(s.client, s.conf, s.log)
	userInfo, err := user.GetUserInfo()
	if err != nil {
		s.log.Error("获取用户信息失败", zap.Error(err))
	} else {
		s.log.With(zap.String("用户", userInfo))
	}
	shopTitle, err := s.SkuTitle()
	if err != nil {
		s.log.Error("获取商品信息失败", zap.Error(err))
	} else {
		s.log.Info("商品名称:", zap.String("", shopTitle))
	}
	url, _ := s.getSeckillUrl()
	skuId := s.conf.SkuId
	req := s.client
	req.SetHeader("User-Agent", s.conf.SkuId)
	req.SetHeader("Host", "marathon.jd.com")
	req.SetHeader("Referer", fmt.Sprintf("https://item.jd.com/%s.html", skuId))
	_, _ = req.GetClient().Get(url)
}

func (s *Seckill) SeckillPage() {
	s.log.Info("访问抢购订单结算页面...")
	skuId := s.conf.SkuId
	seckillNum := s.conf.SeckillNum
	req := s.client
	req.SetHeader("User-Agent", s.conf.UserAgent)
	req.SetHeader("Host", "marathon.jd.com")
	req.SetHeader("Referer", fmt.Sprintf("https://item.jd.com/%s.html", skuId))
	_, _ = req.GetClient().Get("https://marathon.jd.com/seckill/seckill.action?skuId=" + skuId + "&num=" + strconv.Itoa(seckillNum) + "&rid=" + strconv.Itoa(int(time.Now().Unix())))
}

func (s *Seckill) SeckillInitInfo() (string, error) {
	s.log.Info("获取秒杀初始化信息...")
	skuId := s.conf.SkuId
	seckillNum := s.conf.SeckillNum
	req := s.client.R()
	req.SetHeader("User-Agent", s.conf.UserAgent)
	req.SetHeader("Host", "marathon.jd.com")
	body := map[string]interface{}{
		"sku":             skuId,
		"num":             seckillNum,
		"isModifyAddress": "false",
	}
	req.SetBody(body)
	resp, err := req.Post("https://marathon.jd.com/seckillnew/orderService/pc/init.action")
	if err != nil {
		s.log.Error("初始化秒杀信息失败")
		return "", errors.New("初始化秒杀信息失败")
	}
	return string(resp.Body()), nil
}

func (s *Seckill) SubmitSeckillOrder() bool {
	eid := s.conf.EID
	fp := s.conf.FP
	skuId := s.conf.SkuId
	seckillNum := s.conf.SeckillNum
	paymentPwd := s.conf.Account
	initInfo, _ := s.SeckillInitInfo()
	address := gjson.Get(initInfo, "addressList").Array()
	defaultAddress := address[0]
	isinvoiceInfo := gjson.Get(initInfo, "invoiceInfo").Exists()
	invoiceTitle := "-1"
	invoiceContentType := "-1"
	invoicePhone := ""
	invoicePhoneKey := ""
	if isinvoiceInfo {
		invoiceTitle = gjson.Get(initInfo, "invoiceInfo.invoiceTitle").String()
		invoiceContentType = gjson.Get(initInfo, "invoiceInfo.invoiceContentType").String()
		invoicePhone = gjson.Get(initInfo, "invoiceInfo.invoicePhone").String()
		invoicePhoneKey = gjson.Get(initInfo, "invoiceInfo.invoicePhoneKey").String()
	}
	invoiceInfo := "false"
	if isinvoiceInfo {
		invoiceInfo = "true"
	}
	token := gjson.Get(initInfo, "token").String()
	s.log.Info("提交抢购订单...")
	req := s.client.NewRequest()
	req.SetHeader("User-Agent", s.conf.UserAgent)
	req.SetHeader("Host", "marathon.jd.com")
	req.SetHeader("Referer", fmt.Sprintf("https://marathon.jd.com/seckill/seckill.action?skuId=%s&num=%s&rid=%d", skuId, seckillNum, int(time.Now().Unix())))
	body := map[string]interface{}{
		"skuId":              skuId,
		"num":                seckillNum,
		"addressId":          defaultAddress.Get("id").String(),
		"yuShou":             "true",
		"isModifyAddress":    "false",
		"name":               defaultAddress.Get("name").String(),
		"provinceId":         defaultAddress.Get("provinceId").String(),
		"cityId":             defaultAddress.Get("cityId").String(),
		"countyId":           defaultAddress.Get("countyId").String(),
		"townId":             defaultAddress.Get("townId").String(),
		"addressDetail":      defaultAddress.Get("addressDetail").String(),
		"mobile":             defaultAddress.Get("mobile").String(),
		"mobileKey":          defaultAddress.Get("mobileKey").String(),
		"email":              defaultAddress.Get("email").String(),
		"postCode":           "",
		"invoiceTitle":       invoiceTitle,
		"invoiceCompanyName": "",
		"invoiceContent":     invoiceContentType,
		"invoiceTaxpayerNO":  "",
		"invoiceEmail":       "",
		"invoicePhone":       invoicePhone,
		"invoicePhoneKey":    invoicePhoneKey,
		"invoice":            invoiceInfo,
		"password":           paymentPwd,
		"codTimeType":        "3",
		"paymentType":        "4",
		"areaCode":           "",
		"overseas":           "0",
		"phone":              "",
		"eid":                eid,
		"fp":                 fp,
		"token":              token,
		"pru":                "",
	}

	resp, err := req.Post("https://marathon.jd.com/seckillnew/orderService/pc/submitOrder.action?skuId=" + skuId)
	if err != nil || resp.StatusCode() != http.StatusOK {
		s.log.Error("抢购失败，网络错误")
		s.sendEmail()
		return false
	}
	respBody := string(resp.Body())
	if !gjson.Valid(respBody) {
		s.log.Error("抢购失败", zap.Any("body", body))
		s.sendEmail()
		return false
	}
	if gjson.Get(respBody, "success").Bool() {
		orderId := gjson.Get(respBody, "orderId").String()
		totalMoney := gjson.Get(respBody, "totalMoney").String()
		payUrl := "https:" + gjson.Get(respBody, "pcUrl").String()
		s.log.Info(fmt.Sprintf("抢购成功，订单号:%s, 总价:%s, 电脑端付款链接:%s", orderId, totalMoney, payUrl))
		s.sendEmail()
		return true
	} else {
		s.log.Info("抢购失败，返回信息:" + respBody)
		s.sendEmail()
		return false
	}
}

func (s *Seckill) sendEmail() {
	if s.conf.Message.Enable && s.conf.Message.Type == "smtp" {
		email := email2.NewEmail(s.conf.Smtp, s.log)
		_ = email.SendMail([]string{s.conf.Message.Email}, "抢购通知", "抢购失败，网络错误")
	}
}

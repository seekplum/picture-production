package main

import (
	"flag"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"time"

	"bytes"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
)

const DEFAULT_MAX_WIDTH float64 = 300
const DEFAULT_MAX_HEIGHT float64 = 300

type imgResponse struct {
	code int
	msg  string
	buf  *bytes.Buffer
}

// 计算图片缩放后的尺寸
func calculateRatioFit(srcWidth, srcHeight int) (int, int) {
	ratio := math.Min(DEFAULT_MAX_WIDTH/float64(srcWidth), DEFAULT_MAX_HEIGHT/float64(srcHeight))
	return int(math.Ceil(float64(srcWidth) * ratio)), int(math.Ceil(float64(srcHeight) * ratio))
}

func getImgSize(imgPath string) (int, int, image.Image, error) {
	imageFile, err := os.Open(imgPath)
	if err != nil {
		return 0, 0, nil, err
	}
	defer imageFile.Close()
	img, err := png.Decode(imageFile)
	if err != nil {
		return 0, 0, nil, err
	}
	// 获取图片大小
	b := img.Bounds()
	width := b.Max.X
	height := b.Max.Y
	return height, width, img, nil
}

func resizeImg(imgPath string) (*bytes.Buffer, error) {
	height, width, img, err := getImgSize(imgPath)
	if err != nil {
		return nil, err
	}
	// 图片尺寸不对时进行调整
	if float64(width) == DEFAULT_MAX_WIDTH && float64(height) == DEFAULT_MAX_HEIGHT {
		content, err := ioutil.ReadFile(imgPath)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(content), nil
	}
	w, h := calculateRatioFit(width, height)
	// 调用resize库进行图片缩放
	m := resize.Resize(uint(w), uint(h), img, resize.Lanczos3)
	buf := new(bytes.Buffer)
	// 以PNG格式保存文件
	err = png.Encode(buf, m)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func generateAvatar() imgResponse {
	// 获取当前路径
	pwd, _ := os.Getwd()
	// 图片路径
	imgDir := path.Join(pwd, "images")
	// 300 * 300 的背景头像图
	originBackgroundPath := path.Join(imgDir, "avatar.png")
	backgroundBuf, err := resizeImg(originBackgroundPath)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10011,
			msg:  err.Error(),
		}
	}
	resizeBackgroundImg, err := png.Decode(backgroundBuf)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10013,
			msg:  err.Error(),
		}
	}

	// 300 * 300 前景红旗图
	foregroundPath := path.Join(imgDir, "hat.png")
	foregroundImageFile, err := os.Open(foregroundPath)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10021,
			msg:  err.Error(),
		}
	}
	defer foregroundImageFile.Close()
	foregroundImage, err := png.Decode(foregroundImageFile)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10022,
			msg:  err.Error(),
		}
	}

	// 初始化一个画布
	newPng := image.NewRGBA(image.Rect(0, 0, 300, 300))

	// 先绘制背景图
	draw.Draw(newPng,
		newPng.Bounds(),
		resizeBackgroundImg,
		resizeBackgroundImg.Bounds().Min,
		draw.Over,
	)
	// 再绘制前景图
	draw.Draw(newPng,
		newPng.Bounds(),
		foregroundImage,
		foregroundImage.Bounds().Min.Sub(image.Pt(0, 0)),
		draw.Over,
	)

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, newPng); nil != err {
		log.Println("output new image with error: ", err)
		return imgResponse{
			code: 10032,
			msg:  err.Error(),
		}
	}

	return imgResponse{
		code: 0,
		msg:  "",
		buf:  buf,
	}
}

func generateAvatarForBase64(c *gin.Context) {
	response := generateAvatar()
	if response.code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": response.code, "msg": response.msg})
		return
	}
	// 转成 string
	base64Str := base64.StdEncoding.EncodeToString(response.buf.Bytes())

	// 处理图片格式
	imgBase64 := "data:image/png;base64," + base64Str
	c.JSON(http.StatusOK, gin.H{
		"code":   0,
		"base64": imgBase64,
	})
	return

}

func generateAvatarForImg(c *gin.Context) {
	response := generateAvatar()
	if response.code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": response.code, "msg": response.msg})
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.Data(http.StatusOK, "application/octet-stream", response.buf.Bytes())
	return
}

func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		log.Printf("method: %s, uri: %s, duration: %s", c.Request.Method, c.Request.URL.Path, duration)
	}
}

func getEnvDefault(key, defaultVal string) string {
	val, ex := os.LookupEnv(key)
	if !ex {
		return defaultVal
	}
	return val
}

func main() {
	// 设置日志格式
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	log.SetOutput(os.Stderr)

	defaultHost := getEnvDefault("HOST", ":8089")
	var host string
	flag.StringVar(&host, "host", defaultHost, "绑定的端口号")
	flag.Parse()

	r := gin.New()
	r.Use(RequestLoggerMiddleware())
	r.GET("/render/base64", generateAvatarForBase64)
	r.GET("/render/img", generateAvatarForImg)
	r.Run(host)
}

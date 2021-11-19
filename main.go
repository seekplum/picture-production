package main

import (
	"errors"
	"flag"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"bytes"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nfnt/resize"
)

const DEFAULT_MAX_WIDTH float64 = 300
const DEFAULT_MAX_HEIGHT float64 = 300
const HAT_IMAGE_NAME string = "hat.png"
const DEMO_IMAGE_NAME string = "demo.png"

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

func getImageDirectory() string {
	// 获取当前路径
	pwd, _ := os.Getwd()
	// 图片路径
	imgDir := path.Join(pwd, "images")
	return imgDir
}

func getImgSize(imgBuf *bytes.Buffer) (int, int, image.Image, error) {
	img, err := png.Decode(imgBuf)
	if err != nil {
		return 0, 0, nil, err
	}
	// 获取图片大小
	b := img.Bounds()
	width := b.Max.X
	height := b.Max.Y
	return height, width, img, nil
}

func resizeImg(imgBuf *bytes.Buffer) (*bytes.Buffer, error) {
	height, width, img, err := getImgSize(imgBuf)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	// 图片尺寸不对时进行调整
	if float64(width) == DEFAULT_MAX_WIDTH && float64(height) == DEFAULT_MAX_HEIGHT {
		// PNG 数据写入 buffer
		err = png.Encode(buf, img)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
	w, h := calculateRatioFit(width, height)
	// 调用resize库进行图片缩放
	m := resize.Resize(uint(w), uint(h), img, resize.Lanczos3)
	// PNG 数据写入 buffer
	err = png.Encode(buf, m)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func isPng(r io.Reader) error {
	const pngHeader = "\x89PNG\r\n\x1a\n"
	const headLength = len(pngHeader)
	var tmp [headLength]byte
	_, err := io.ReadFull(r, tmp[:headLength])
	if err != nil {
		return err
	}
	if string(tmp[:headLength]) != pngHeader {
		return errors.New("not a PNG file")
	}
	return nil
}

func generateAvatar(imgBuf *bytes.Buffer) imgResponse {
	backgroundBuf, err := resizeImg(imgBuf)
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
	foregroundPath := path.Join(getImageDirectory(), HAT_IMAGE_NAME)
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

func covertImage(content []byte) (*bytes.Buffer, error) {
	pngErr := isPng(bytes.NewBuffer(content))
	// PNG 格式直接返回
	if pngErr == nil {
		return bytes.NewBuffer(content), nil
	}
	// 读取 jpg 格式内容写入 png 中
	img, err := jpeg.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	// jpg 格式转成 png 格式
	if err := png.Encode(buf, img); err != nil {
		return nil, err
	}
	return buf, nil
}

func getDemoImageBuffer() (*bytes.Buffer, error) {
	// 300 * 300 的背景头像图
	demoPath := path.Join(getImageDirectory(), DEMO_IMAGE_NAME)
	content, err := ioutil.ReadFile(demoPath)
	if err != nil {
		return nil, err
	}
	return covertImage(content)
}

func getUploadImage(c *gin.Context) (*bytes.Buffer, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return nil, err
	}
	fileExt := strings.ToLower(path.Ext(file.Filename))
	if fileExt != ".png" && fileExt != ".jpg" && fileExt != ".jpeg" {
		return nil, errors.New("The file type is incorrect. Only PNG/JPG format is supported")
	}
	fileContent, err := file.Open()
	defer fileContent.Close()
	if err != nil {
		return nil, err
	}
	fileBytes, err := ioutil.ReadAll(fileContent)
	if err != nil {
		return nil, err
	}
	return covertImage(fileBytes)
}

func renderBase64(imgBuf *bytes.Buffer, c *gin.Context) {
	response := generateAvatar(imgBuf)
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
func renderImgFile(imgBuf *bytes.Buffer, c *gin.Context) {
	response := generateAvatar(imgBuf)
	if response.code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": response.code, "msg": response.msg})
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.Data(http.StatusOK, "application/octet-stream", response.buf.Bytes())
	return
}

func generateDemoAvatarForBase64(c *gin.Context) {
	demoBuf, err := getDemoImageBuffer()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10031, "msg": err.Error()})
		return
	}
	renderBase64(demoBuf, c)
	return
}

func generateDemoAvatarForImg(c *gin.Context) {
	demoBuf, err := getDemoImageBuffer()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10041, "msg": err.Error()})
		return
	}
	renderImgFile(demoBuf, c)
	return
}

func generateAvatarForBase64(c *gin.Context) {
	imgBuf, err := getUploadImage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10051, "msg": err.Error()})
		return
	}
	renderBase64(imgBuf, c)
	return
}

func generateAvatarForImg(c *gin.Context) {
	imgBuf, err := getUploadImage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10061, "msg": err.Error()})
		return
	}
	renderImgFile(imgBuf, c)
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
	r.GET("/render/demo/base64", generateDemoAvatarForBase64)
	r.GET("/render/demo/img", generateDemoAvatarForImg)
	r.POST("/render/base64", generateAvatarForBase64)
	r.POST("/render/img", generateAvatarForImg)
	r.Run(host)
}

package main

import (
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"

	"bufio"
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
	path string
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

func resizeImg(imgPath, resizeImgPath string) (string, error) {
	height, width, img, err := getImgSize(imgPath)
	if err != nil {
		return "", err
	}
	// 图片尺寸不对时进行调整
	if float64(width) == DEFAULT_MAX_WIDTH && float64(height) == DEFAULT_MAX_HEIGHT {
		return imgPath, nil
	}
	w, h := calculateRatioFit(width, height)
	// 调用resize库进行图片缩放
	m := resize.Resize(uint(w), uint(h), img, resize.Lanczos3)
	// 需要保存的文件
	resizeImgFile, err := os.Create(resizeImgPath)
	if err != nil {
		return "", err
	}
	defer resizeImgFile.Close()
	// 以PNG格式保存文件
	err = png.Encode(resizeImgFile, m)
	if err != nil {
		return "", err
	}
	return resizeImgPath, nil
}

func generateAvatar(tempDir string) imgResponse {
	// 获取当前路径
	pwd, _ := os.Getwd()
	// 图片路径
	imgDir := path.Join(pwd, "images")

	// 300 * 300 的背景头像图
	originBackgroundPath := path.Join(imgDir, "avatar.png")
	resizeBackgroundPath := path.Join(tempDir, "resize_avatar.png")
	backgroundImagePath, err := resizeImg(originBackgroundPath, resizeBackgroundPath)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10011,
			msg:  err.Error(),
		}
	}
	backgroundImageFile, err := os.Open(backgroundImagePath)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10012,
			msg:  err.Error(),
		}
	}
	defer backgroundImageFile.Close()
	resizeBackgroundImg, err := png.Decode(backgroundImageFile)
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

	dstFIlePath := path.Join(tempDir, "dst.png")
	// 目标图片
	dstFile, err := os.Create(dstFIlePath)
	if err != nil {
		log.Println(err)
		return imgResponse{
			code: 10031,
			msg:  err.Error(),
		}
	}
	defer dstFile.Close()
	if err := png.Encode(dstFile, newPng); nil != err {
		log.Println("output new image with error: ", err)
		return imgResponse{
			code: 10032,
			msg:  err.Error(),
		}
	}

	return imgResponse{
		code: 0,
		msg:  "",
		path: dstFIlePath,
	}
}

func generateAvatarForBase64(c *gin.Context) {
	tempDir, err := ioutil.TempDir("", "gen-avatar-base64")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10041, "msg": err.Error()})
		return
	}
	defer os.RemoveAll(tempDir)
	response := generateAvatar(tempDir)
	if response.code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": response.code, "msg": response.msg})
		return
	}
	imgFile, err := os.Open(response.path)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 10042, "msg": err.Error()})
		return
	}

	// 根据文件大小创建一个新的缓冲区
	imgInfo, _ := imgFile.Stat()
	var size int64 = imgInfo.Size()
	buf := make([]byte, size)

	// 将文件内容读入缓冲区
	imgReader := bufio.NewReader(imgFile)
	imgReader.Read(buf)

	// 转成 string
	base64Str := base64.StdEncoding.EncodeToString(buf)

	// 处理图片格式
	imgBase64 := "data:image/png;base64," + base64Str
	c.JSON(http.StatusOK, gin.H{
		"code":   0,
		"base64": imgBase64,
	})
	return

}

func generateAvatarForImg(c *gin.Context) {
	tempDir, err := ioutil.TempDir("", "gen-avatar-img")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10051, "msg": err.Error()})
		return
	}
	defer os.RemoveAll(tempDir)
	response := generateAvatar(tempDir)
	if response.code != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": response.code, "msg": response.msg})
		return
	}
	c.Header("Content-Type", "image/jpeg")
	c.File(response.path)
	return
}

func main() {
	// 设置日志格式
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	log.SetOutput(os.Stderr)
	r := gin.New()
	r.GET("/render/base64", generateAvatarForBase64)
	r.GET("/render/img", generateAvatarForImg)
	r.Run(":8000")
}

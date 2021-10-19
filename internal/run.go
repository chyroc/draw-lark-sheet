package internal

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strconv"

	"github.com/chyroc/go-ptr"
	"github.com/chyroc/lark"
	"github.com/chyroc/lark/larkext"
	imagedraw "golang.org/x/image/draw"
)

type Request struct {
	LarkAppID     string
	LarkAppSecret string
	LarkUserID    string
	LarkSheet     string
	ImagePath     string
}

func Run(request Request) error {
	ctx := context.Background()

	image, err := parseImage(request.ImagePath, 100)
	if err != nil {
		return err
	}

	imageColor := combineImageColor(image)

	larkClient := lark.New(lark.WithAppCredential(request.LarkAppID, request.LarkAppSecret))
	sheetClient, err := makeSheetClient(ctx, larkClient, request.LarkSheet, request.LarkUserID)
	if err != nil {
		return err
	}
	fmt.Println("创建 Sheet: ", sheetClient.SheetToken())

	return drawSheet(ctx, sheetClient, imageColor)
}

func makeSheetClient(ctx context.Context, larkClient *lark.Lark, sheetToken, assignUserID string) (*larkext.Sheet, error) {
	if sheetToken == "" {
		folderClient := larkext.NewFolder(larkClient, "")

		sheetClient, err := folderClient.NewSheet(ctx, "draw-lark-sheet")
		if err != nil {
			return nil, err
		}

		_, _, err = larkClient.Drive.UpdateDriveMemberPermission(ctx, &lark.UpdateDriveMemberPermissionReq{
			NeedNotification: ptr.Bool(true),
			Type:             "sheet",
			Token:            sheetClient.SheetToken(),
			MemberID:         assignUserID,
			MemberType:       "userid",
			Perm:             "full_access",
		})
		if err != nil {
			return nil, err
		}

		// 保证有 100 x 100 个格子，且每个格子是 6x6 的

		meta, err := sheetClient.Meta(ctx)
		if err != nil {
			return nil, err
		}
		sheet := meta.Sheets[0]
		if sheet.RowCount > 100 {
			if err = sheetClient.DeleteRows(ctx, sheet.SheetID, 101, int(sheet.RowCount-100)); err != nil {
				return nil, err
			}
		} else if sheet.RowCount < 100 {
			if err = sheetClient.AddRows(ctx, sheet.SheetID, int(100-sheet.RowCount)); err != nil {
				return nil, err
			}
		}

		if sheet.ColumnCount > 100 {
			if err = sheetClient.DeleteCols(ctx, sheet.SheetID, 101, int(sheet.ColumnCount-100)); err != nil {
				return nil, err
			}
		} else if sheet.ColumnCount < 100 {
			if err = sheetClient.AddCols(ctx, sheet.SheetID, int(100-sheet.ColumnCount)); err != nil {
				return nil, err
			}
		}

		if err = sheetClient.SetRowsSize(ctx, sheet.SheetID, 1, 100, 6); err != nil {
			return nil, err
		}

		if err = sheetClient.SetColsSize(ctx, sheet.SheetID, 1, 100, 6); err != nil {
			return nil, err
		}

		sheetToken = sheetClient.SheetToken()
	}

	return larkext.NewSheet(larkClient, sheetToken), nil
}

func parseImage(imagePath string, size int) (image.Image, error) {
	fmt.Println("输入图片: ", imagePath)

	reader, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, err
	}

	scale := imagedraw.NearestNeighbor

	dx := img.Bounds().Dx()
	dy := img.Bounds().Dy()
	maxd := max(dx, dy)
	if maxd > size {
		newDx := dx * size / maxd
		newDy := dy * size / maxd
		dst := image.NewRGBA(image.Rectangle{Min: image.Point{}, Max: image.Point{X: newDx, Y: newDy}})
		scale.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
		img = dst
	}

	return img, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// []map[[4]int]string
func combineImageColor(image image.Image) map[string][][4]int {
	bounds := image.Bounds()

	result := map[string][][4]int{}
	for x := 0; x < bounds.Size().X; x++ {
		key := [4]int{}
		prefixColor := ""
		for y := 0; y < bounds.Size().Y; y++ {
			r, g, b, _ := image.At(x, y).RGBA()
			pointColor := rgbToHex(r>>8, g>>8, b>>8)
			if prefixColor == "" {
				// 第一次遇到，这是起点
				prefixColor = pointColor
				key = [4]int{x, y, x, y}
			} else {
				if prefixColor == pointColor {
					// 第二次遇到相同值，这是终点 +1
					key[3] = y
				} else {
					// 遇到了不同值，前一个需要结束，现在这个需要开始
					result[prefixColor] = append(result[prefixColor], key)
					prefixColor = pointColor
					key = [4]int{x, y, x, y}
				}
			}

			if y == bounds.Size().Y-1 {
				// 最后一个，结束
				result[prefixColor] = append(result[prefixColor], key)
			}
		}
	}

	return result
}

func drawSheet(ctx context.Context, sheetClient *larkext.Sheet, colors map[string][][4]int) error {
	meta, err := sheetClient.Meta(ctx)
	if err != nil {
		return err
	}
	sheetID := meta.Sheets[0].SheetID
	styles := []*lark.BatchSetSheetStyleReqData{}
	for color, cells := range colors {
		if color == "#ffffff" {
			continue
		}
		cellRanges := []string{}
		for _, v := range cells {
			cellRange := larkext.CellRange(sheetID, v[0]+1, v[1]+1, v[2]+1, v[3]+1)
			cellRanges = append(cellRanges, cellRange)
		}

		styles = append(styles, &lark.BatchSetSheetStyleReqData{
			Ranges: cellRanges,
			Style:  &lark.BatchSetSheetStyleReqDataStyle{BackColor: ptr.String(color)},
		})
	}

	fmt.Println("写入 Sheet:", len(styles))
	err = sheetClient.BatchSetCellStyle(ctx, styles)

	return err
}

type combineColor struct {
	Point [4]int
	Color string
}

func toHex(r uint32) string {
	x := strconv.FormatInt(int64(r), 16)
	if len(x) == 1 {
		return "0" + x
	}
	return x
}

func rgbToHex(r, g, b uint32) string {
	return "#" + toHex(r) + toHex(g) + toHex(b)
}

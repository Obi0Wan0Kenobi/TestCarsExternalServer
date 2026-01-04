package main

import (
	"github.com/gofiber/fiber/v2/middleware/compress"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ExternalCarDto struct {
	ExternalId      string  `json:"externalId"`
	Brand           string  `json:"brand"`
	Model           string  `json:"model"`
	Year            int     `json:"year"`
	Price           float64 `json:"price"`
	SourceUpdatedAt string  `json:"sourceUpdatedAt"`
}

var totalCount atomic.Int64      //сколько всего машин отдаём
var updatedCount atomic.Int64    //сколько первых машин считаем "обновлёнными" (sourceUpdatedAt новее)
var baseVersionDays atomic.Int64 //базовый сдвиг времени (в днях) для ВСЕХ машин
var bumpDays atomic.Int64        //насколько "обновлённые" новее (в днях)

func main() {
	//default values
	totalCount.Store(10000)  //сколько всего автомобилей выдать
	updatedCount.Store(0)    //у скольких автомобилей sourceUpdatedAt будет свежее
	baseVersionDays.Store(0) //на сколько свежее машины будут при обычном запросе (в днях)
	bumpDays.Store(1)        //обновлённые машины будут свежее на 1 день (в днях)

	app := fiber.New()

	//gzip middleware, решил добавить сжатие ответа :)
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	//POST /values/set
	//count - сколько авто будет выводиться
	//updated - у скольки авто sourceUpdatedAt будет свежее
	//versionDays - на сколько свежее машины будут при обычном запросе (в днях)
	//bumpDays - на сколько обновлённые машины будут свежее (в днях)
	app.Post("/values/set", func(c *fiber.Ctx) error {
		if qs := c.Query("count"); qs != "" {
			n, err := strconv.ParseInt(qs, 10, 64)
			if err != nil || n < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "bad count")
			}
			totalCount.Store(n)
		}

		if qs := c.Query("updated"); qs != "" {
			u, err := strconv.ParseInt(qs, 10, 64)
			if err != nil || u < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "bad updated")
			}
			updatedCount.Store(u)
		}

		if qs := c.Query("versionDays"); qs != "" {
			v, err := strconv.ParseInt(qs, 10, 64)
			if err != nil || v < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "bad versionDays")
			}
			baseVersionDays.Store(v)
		}

		if qs := c.Query("bumpDays"); qs != "" {
			b, err := strconv.ParseInt(qs, 10, 64)
			if err != nil || b < 0 {
				return fiber.NewError(fiber.StatusBadRequest, "bad bumpDays")
			}
			bumpDays.Store(b)
		}

		tc := totalCount.Load()
		uc := updatedCount.Load()
		if uc > tc {
			updatedCount.Store(tc)
			uc = tc
		}

		return c.JSON(fiber.Map{
			"count":       tc,
			"updated":     uc,
			"versionDays": baseVersionDays.Load(),
			"bumpDays":    bumpDays.Load(),
		})
	})

	//GET /cars
	//возвращает список машин
	//параметры запроса:
	//count - переопределяет количество машин в ответе
	//updated - переопределяет количество обновлённых машин в ответе
	//остальные параметры (versionDays, bumpDays) берутся из глобальных настроек
	app.Get("/cars", func(c *fiber.Ctx) error {
		n := totalCount.Load()
		u := updatedCount.Load()
		vDays := baseVersionDays.Load()
		bDays := bumpDays.Load()

		//override per request (удобно в тестах, без admin/set)
		if qs := c.Query("count"); qs != "" {
			if x, err := strconv.ParseInt(qs, 10, 64); err == nil && x >= 0 {
				n = x
			}
		}
		if qs := c.Query("updated"); qs != "" {
			if x, err := strconv.ParseInt(qs, 10, 64); err == nil && x >= 0 {
				u = x
			}
		}

		if u > n {
			u = n
		}

		base := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC).Add(time.Duration(vDays) * 24 * time.Hour)
		updatedBase := base.Add(time.Duration(bDays) * 24 * time.Hour) //“обновлённые” новее на bumpDays

		brands := []string{"Audi", "BMW", "Toyota", "Honda", "Ford", "Kia", "Hyundai"}
		models := []string{"A6", "X5", "Camry", "Civic", "Focus", "Sportage", "Elantra"}

		cars := make([]ExternalCarDto, 0, int(n))
		for i := int64(1); i <= n; i++ {
			t := base.Add(time.Duration(i%86400) * time.Second)
			if i <= u {
				t = updatedBase.Add(time.Duration(i%86400) * time.Second)
			}
			dto := ExternalCarDto{
				ExternalId:      "jp-" + prefixCar(i),
				Brand:           brands[i%int64(len(brands))],
				Model:           models[i%int64(len(models))],
				Year:            1990 + int(i%35),
				Price:           1000.0 + float64(i%50000)/10.0,
				SourceUpdatedAt: t.Format(time.RFC3339),
			}

			cars = append(cars, dto)
		}
		return c.JSON(cars)
	})

	err := app.Listen(":8081")
	if err != nil {
		log.Fatal(err)
	}
}

func prefixCar(i int64) string {
	s := strconv.FormatInt(i, 10)
	for len(s) < 6 {
		s = "0" + s
	}
	return s
}

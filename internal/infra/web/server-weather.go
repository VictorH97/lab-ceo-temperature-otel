package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ServerWeather struct {
	WeatherAPIKey         string
	LocaleRequestNameOTEL string
	TempRequestNameOTEL   string
	OTELTracer            trace.Tracer
}

type Weather struct {
	Location struct {
		Name string `json:"name"`
	} `json:"location"`
	Current struct {
		TempC float64 `json:"temp_c"`
		TempF float64 `json:"temp_f"`
	} `json:"current"`
}

type Temperatures struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

func NewServerWeather(localeRequestNameOTEL string, tempRequestNameOTEL string, otelTracer trace.Tracer, weatherAPIKey string) *ServerWeather {
	return &ServerWeather{
		LocaleRequestNameOTEL: localeRequestNameOTEL,
		TempRequestNameOTEL:   tempRequestNameOTEL,
		OTELTracer:            otelTracer,
		WeatherAPIKey:         weatherAPIKey,
	}
}

func (we *ServerWeather) CreateServerWeather() *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Logger)
	router.Use(middleware.Timeout(60 * time.Second))
	// promhttp
	router.Handle("/metrics", promhttp.Handler())
	router.Get("/", we.FindTemperature)
	return router
}

func (h *ServerWeather) FindTemperature(w http.ResponseWriter, r *http.Request) {
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := r.Context()
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	ctx, spanLocale := h.OTELTracer.Start(ctx, h.LocaleRequestNameOTEL)

	cepParam := r.URL.Query().Get("cep")
	if cepParam == "" {
		http.Error(w, "Cep is required", http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	valid, err := VerifyValidCEP(cepParam)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !valid {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	cep, err := h.GetCEPInfo(cepParam)
	if err != nil {
		if err.Error() == "can not find zipcode" {
			http.Error(w, err.Error(), http.StatusNotFound)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		http.Error(w, "Error getting CEP info: "+err.Error(), http.StatusBadRequest)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	spanLocale.End()

	_, spanTemperature := h.OTELTracer.Start(ctx, h.TempRequestNameOTEL)
	defer spanTemperature.End()

	weather, err := h.GetWeatherInfo(cep.Localidade, h.WeatherAPIKey)
	if err != nil {
		http.Error(w, "Error getting weather info: "+err.Error(), http.StatusBadRequest)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	temperatures := Temperatures{
		City:  weather.Location.Name,
		TempC: weather.Current.TempC,
		TempF: weather.Current.TempF,
		TempK: weather.Current.TempC + 273,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(temperatures)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (h *ServerWeather) GetCEPInfo(cep string) (*ViaCEP, error) {
	req, err := http.NewRequest("GET", "http://viacep.com.br/ws/"+cep+"/json/", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "erro") {
		return nil, errors.New("can not find zipcode")
	}

	var cepData ViaCEP

	err = json.Unmarshal(body, &cepData)
	if err != nil {
		return nil, err
	}

	return &cepData, nil
}

func (h *ServerWeather) GetWeatherInfo(location string, apiKey string) (*Weather, error) {
	requestUrl := "http://api.weatherapi.com/v1/current.json?key=" + apiKey + "&q=" + url.QueryEscape(location)

	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "erro") {
		var errorResponse struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		err = json.Unmarshal(body, &errorResponse)
		if err != nil {
			return nil, err
		}

		return nil, errors.New(errorResponse.Error.Message)
	}

	var weather Weather

	err = json.Unmarshal(body, &weather)

	if err != nil {
		return nil, err
	}

	return &weather, nil
}

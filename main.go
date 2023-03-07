package main

import (
	"log"
	"net/http"

	"github.com/kahlys/proxy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

var mask = []byte{'G', 'r', 'o', 'w', 'a', 't', 't'}

func run() error {
	mVoltage := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inverter_voltage_volts",
		Help: "Measures voltage on various circuits in volts",
	}, []string{"serial", "type"})
	mPower := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inverter_power_watts",
		Help: "Measures power on various in watts",
	}, []string{"serial", "type"})
	mLastSeen := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inverter_last_seen_unix_timestamp",
		Help: "When was the inverter last seen",
	}, []string{"serial"})
	mBatterySOC := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "inverter_battery_state_of_charge",
		Help: "What is the battery charge",
	}, []string{"serial"})

	// Create non-global registry.
	registry := prometheus.NewRegistry()

	// Add go runtime metrics and process collectors.
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		mVoltage,
		mPower,
		mLastSeen,
		mBatterySOC,
	)

	// Expose /metrics HTTP endpoint using the created custom registry.
	http.Handle(
		"/metrics", promhttp.HandlerFor(
			registry,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			}),
	)
	go func() {
		log.Fatalln(http.ListenAndServe(":5280", nil))
	}()

	s := &proxy.Server{
		Addr:   ":5279",
		Target: "server.growatt.com:5279",
		ModifyRequest: func(req *[]byte) {
			input := make([]byte, len(*req))
			copy(input, *req)

			var (
				msg Msg
				d   Data
			)
			if err := decode(input, &msg); err != nil {
				log.Printf("error decoding request message: %s", err)
				return
			}

			log.Printf("request id=%v type=%v length=%v ", msg.ID, msg.Type, msg.Length)

			if msg.Type != MsgTypeData {
				return
			}
			if err := decodeDataMessage(&msg, &d); err != nil {
				log.Printf("error decoding request message data: %s", err)
				return
			}
			log.Printf("data request inverter=%v soc=%v power=%v ", d.Inverter, d.SOC, d.PACToUserR)

			mLastSeen.WithLabelValues(d.Serial).Set(float64(d.Timestamp.UnixNano()) / 1e9)
			mPower.WithLabelValues(d.Serial, "battery-charge").Set(d.PCharge)
			mPower.WithLabelValues(d.Serial, "battery-discharge").Set(d.PDischarge)
			mPower.WithLabelValues(d.Serial, "to-user").Set(d.PACToUserR)
			mPower.WithLabelValues(d.Serial, "pv1").Set(float64(d.PV1.Power) * 0.1)
			mPower.WithLabelValues(d.Serial, "pv2").Set(float64(d.PV2.Power) * 0.1)
			mVoltage.WithLabelValues(d.Serial, "battery").Set(d.VBat)
			mVoltage.WithLabelValues(d.Serial, "grid").Set(d.GridVoltage)
			mVoltage.WithLabelValues(d.Serial, "pv1").Set(float64(d.PV1.Voltage) * 0.1)
			mVoltage.WithLabelValues(d.Serial, "pv2").Set(float64(d.PV2.Voltage) * 0.1)
			mBatterySOC.WithLabelValues(d.Serial).Set(d.SOC)
		},
		ModifyResponse: func(resp *[]byte) {
			input := make([]byte, len(*resp))
			copy(input, *resp)
			var (
				msg Msg
			)
			if err := decode(input, &msg); err != nil {
				log.Printf("error decoding response message: %s", err)
				return
			}
			log.Printf("response id=%v type=%v length=%v ", msg.ID, msg.Type, msg.Length)
		},
	}

	return s.ListenAndServe()
}

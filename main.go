package main

import (
	"fmt"
	"os"

	teleinfo "github.com/ferllings/goteleinfo"
	eria "github.com/project-eria/eria-core"
	"github.com/project-eria/go-wot/dataSchema"
	"github.com/project-eria/go-wot/interaction"
	"github.com/project-eria/go-wot/thing"
	zlog "github.com/rs/zerolog/log"
)

func readFrames(reader teleinfo.Reader, framesChan chan<- teleinfo.Frame) {
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			zlog.Warn().Err(err).Msg("[main] Error reading Teleinfo frame")
			continue
		}
		framesChan <- frame
	}
}

var config = struct {
	Host        string `yaml:"host"`
	Port        uint   `yaml:"port" default:"80"`
	ExposedAddr string `yaml:"exposedAddr"`
	SerialPort  string `yaml:"serialPort" required:"true"`
	Mode        string `yaml:"mode" default:"historic"` // "standard"
}{}

func convertMap(mapString map[string]string) map[string]interface{} {
	mapInterface := make(map[string]interface{})
	for key, value := range mapString {
		mapInterface[key] = value
	}
	return mapInterface
}

func main() {
	defer func() {
		zlog.Info().Msg("[main] Stopped")
	}()

	eria.Init("ERIA Teleinfo Gateway")
	// Loading config
	eria.LoadConfig(&config)

	port, err := teleinfo.OpenPort(config.SerialPort, config.Mode)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer port.Close()
	reader := teleinfo.NewReader(port, &config.Mode)

	initialFrame, err := reader.ReadFrame()
	if err != nil {
		zlog.Panic().Err(err).Msg("[main] Error reading Teleinfo frame")
	}

	eriaServer := eria.NewServer(config.Host, config.Port, config.ExposedAddr, "")

	td := setThings(initialFrame)

	eriaThing, _ := eriaServer.AddThing("", td)

	framesChan := make(chan teleinfo.Frame, 10)

	// Read Teleinfo frames and send them into framesChan
	go readFrames(reader, framesChan)

	go func() {
		for frame := range framesChan {
			frameMap := frame.GetMap()
			f := convertMap(frameMap)
			eriaThing.SetPropertyValue("raw", f)
			baseValue, _ := frame.GetUIntField("BASE")
			eriaThing.SetPropertyValue("indexBase", int(baseValue))
			iinstValue, _ := frame.GetUIntField("IINST")
			eriaThing.SetPropertyValue("iinst", int(iinstValue))
			pappValue, _ := frame.GetUIntField("PAPP")
			eriaThing.SetPropertyValue("papp", int(pappValue))
		}
	}()

	eriaServer.StartServer()
}

func setThings(frame teleinfo.Frame) *thing.Thing {
	ticType := frame.Type()
	ticMode := frame.Mode()

	td, _ := eria.NewThingDescription(
		"eria:gateway:tic",
		eria.AppVersion,
		"Teleinfo",
		"Teleinfo gateway",
		[]string{},
	)

	rawData := dataSchema.NewObject(map[string]interface{}{})

	property := interaction.NewProperty(
		"raw",
		"Raw data",
		"Raw data in json format",
		true,
		false,
		true,
		rawData,
	)
	td.AddProperty(property)

	if ticMode == "historic" {
		// ADCO     : Adresse du compteur
		// OPTARIF  : Option tarifaire choisie
		// ISOUSC   : Intensité souscrite (A)
		// BASE     : Index option Base (KWh)
		// HCHC     : Index Heures Creuses (KWh)
		// HCHP     : Index Heures Pleines (KWh)
		// EJPHN    : Index option EJP Heures Normales
		// EJPHPM   : Index option EJP Heures de Pointe Mobile
		// BBRHCJB  : Index option Tempo Heures Creuses Jours Bleus
		// BBRHPJB  : Index option Tempo Heures Pleines Jours Bleus
		// BBRHCJW  : Index option Tempo Heures Creuses Jours Blancs
		// BBRHPJW  : Index option Tempo Heures Pleines Jours Blancs
		// BBRHCJR  : Index option Tempo Heures Creuses Jours Rouges
		// BBRHPJR  : Index option Tempo Heures Pleines Jours Rouges
		// PEJP     : Préavis Début EJP
		// PTEC     : Période Tarifaire en cours
		// DEMAIN   : Couleur du lendemain
		// IINST    : Intensité Instantanée (A)
		// ADPS     : Avertissement de Dépassement De Puissance Souscrite
		// IMAX     : Intensité maximale appelée
		// PAPP     : Puissance apparente (VA)
		// HHPHC    : Horaire Heures Pleines Heures Creuses
		// MOTDETAT : Mot d'état du compteur
		if ticType == "BASE" {
			baseValue, _ := frame.GetUIntField("BASE")
			baseData := dataSchema.NewInteger(int(baseValue), "KWh", 0, 1000000000)
			baseProperty := interaction.NewProperty(
				"indexBase",
				"BASE",
				"Index option Base",
				true,
				false,
				true,
				baseData,
			)
			td.AddProperty(baseProperty)
			iinstValue, _ := frame.GetUIntField("IINST")
			iinstData := dataSchema.NewInteger(int(iinstValue), "A", 0, 1000)
			iinstProperty := interaction.NewProperty(
				"iinst",
				"IINST",
				"Intensité Instantanée",
				true,
				false,
				true,
				iinstData,
			)
			td.AddProperty(iinstProperty)
			pappValue, _ := frame.GetUIntField("PAPP")
			pappData := dataSchema.NewInteger(int(pappValue), "VA", 0, 1000)
			pappProperty := interaction.NewProperty(
				"papp",
				"PAPP",
				"Puissance apparente",
				true,
				false,
				true,
				pappData,
			)
			td.AddProperty(pappProperty)
		}
	}

	return td
}

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
	SerialPort string `yaml:"serialPort" required:"true"`
	Mode       string `yaml:"mode" default:"historic"` // "standard"
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

	eria.Init("ERIA Teleinfo Gateway", &config)

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

	td := setThings(initialFrame)

	producer := eria.Producer("")
	eriaThing, _ := producer.AddThing("", td)

	producer.PropertyUseDefaultHandlers(eriaThing, "raw")
	producer.PropertyUseDefaultHandlers(eriaThing, "indexBase")
	producer.PropertyUseDefaultHandlers(eriaThing, "iinst")
	producer.PropertyUseDefaultHandlers(eriaThing, "papp")

	framesChan := make(chan teleinfo.Frame, 10)

	// Read Teleinfo frames and send them into framesChan
	go readFrames(reader, framesChan)

	go func() {
		for frame := range framesChan {
			frameMap := frame.GetMap()
			f := convertMap(frameMap)
			producer.SetPropertyValue(eriaThing, "raw", f)
			baseValue, _ := frame.GetUIntField("BASE")
			producer.SetPropertyValue(eriaThing, "indexBase", int(baseValue))
			iinstValue, _ := frame.GetUIntField("IINST")
			producer.SetPropertyValue(eriaThing, "iinst", int(iinstValue))
			pappValue, _ := frame.GetUIntField("PAPP")
			producer.SetPropertyValue(eriaThing, "papp", int(pappValue))
		}
	}()

	eria.Start("")
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

	rawData, _ := dataSchema.NewObject()

	property := interaction.NewProperty(
		"raw",
		"Raw data",
		"Raw data in json format",
		rawData,
		interaction.PropertyReadOnly(true),
	)
	td.AddProperty(property)

	if ticMode == "historic" {
		// ADCO     : Adresse du compteur
		// OPTARIF  : Option tarifaire choisie
		// ISOUSC   : Intensité souscrite (A)
		// BASE     : Index option Base (Wh)
		// HCHC     : Index Heures Creuses (Wh)
		// HCHP     : Index Heures Pleines (Wh)
		// EJPHN    : Index option EJP Heures Normales (Wh)
		// EJPHPM   : Index option EJP Heures de Pointe Mobile (Wh)
		// BBRHCJB  : Index option Tempo Heures Creuses Jours Bleus (Wh)
		// BBRHPJB  : Index option Tempo Heures Pleines Jours Bleus (Wh)
		// BBRHCJW  : Index option Tempo Heures Creuses Jours Blancs (Wh)
		// BBRHPJW  : Index option Tempo Heures Pleines Jours Blancs (Wh)
		// BBRHCJR  : Index option Tempo Heures Creuses Jours Rouges (Wh)
		// BBRHPJR  : Index option Tempo Heures Pleines Jours Rouges (Wh)
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
			baseData, _ := dataSchema.NewInteger(
				dataSchema.IntegerDefault(int(baseValue)),
				dataSchema.IntegerUnit("%"),
				dataSchema.IntegerMin(0),
				dataSchema.IntegerMax(1000000000),
			)
			baseProperty := interaction.NewProperty(
				"indexBase",
				"BASE",
				"Index option Base",
				baseData,
				interaction.PropertyReadOnly(true),
			)
			td.AddProperty(baseProperty)
			iinstValue, _ := frame.GetUIntField("IINST")
			iinstData, _ := dataSchema.NewInteger(
				dataSchema.IntegerDefault(int(iinstValue)),
				dataSchema.IntegerUnit("A"),
				dataSchema.IntegerMin(0),
				dataSchema.IntegerMax(1000),
			)
			iinstProperty := interaction.NewProperty(
				"iinst",
				"IINST",
				"Intensité Instantanée",
				iinstData,
				interaction.PropertyReadOnly(true),
			)
			td.AddProperty(iinstProperty)
			pappValue, _ := frame.GetUIntField("PAPP")
			pappData, _ := dataSchema.NewInteger(
				dataSchema.IntegerDefault(int(pappValue)),
				dataSchema.IntegerUnit("VA"),
				dataSchema.IntegerMin(0),
				dataSchema.IntegerMax(1000),
			)
			pappProperty := interaction.NewProperty(
				"papp",
				"PAPP",
				"Puissance apparente",
				pappData,
				interaction.PropertyReadOnly(true),
			)
			td.AddProperty(pappProperty)
		}
	}

	return td
}

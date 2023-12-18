package util

type ModelData struct {
	Width  string
	Height string
	Model  string
}

var AndroidVersionToSDK = map[string]string{
	"23": "6",
	"24": "7",
	"25": "7",
	"26": "8",
	"27": "8",
	"28": "9",
	"29": "10",
	"30": "11",
	"31": "12",
	"32": "12",
	"33": "13",
	"34": "14",
}

var IOSDeviceInfoMap = map[string]ModelData{
	// iPhone SE (2nd generation)
	"D79AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone SE (2nd gen)",
	},
	// iPhone 5
	"N41AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5",
	},
	// iPhone 5
	"N42AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5",
	},
	// iPhone 5c
	"N48AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5c",
	},
	// iPhone 5c
	"N49AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5c",
	},
	// iPhone 5s
	"N51AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5S",
	},
	// iPhone 5s
	"N53AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone 5S",
	},
	// iPhone 6
	"N61AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 6",
	},
	// iPhone 6 Plus
	"N56AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 6 Plus",
	},
	// iPhone 6s
	"N71AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 6S",
	},
	// iPhone 6s
	"N71mAP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 6S",
	},
	// iPhone 6s Plus
	"N66AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 6S Plus",
	},
	// iPhone 6s Plus
	"N66mAP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 6S Plus",
	},
	// iPhone SE (1st gen)
	"N69AP": {
		Width:  "320",
		Height: "568",
		Model:  "iPhone SE (1st gen)",
	},
	// iPhone 7
	"D10AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 7",
	},
	// iPhone 7
	"D101AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 7",
	},
	// iPhone 7 Plus
	"D11AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 7 Plus",
	},
	// iPhone 7 Plus
	"D111AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 7 Plus",
	},
	// iPhone 8
	"D20AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 8",
	},
	// iPhone 8
	"D20AAP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 8",
	},
	// iPhone 8
	"D201AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 8",
	},
	// iPhone 8
	"D201AAP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone 8",
	},
	// iPhone 8 plus
	"D21AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 8 Plus",
	},
	// iPhone 8 Plus
	"D21AAP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 8 Plus",
	},
	// iPhone 8 Plus
	"D211AP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 8 Plus",
	},
	// iPhone 8 Plus
	"D211AAP": {
		Width:  "414",
		Height: "736",
		Model:  "iPhone 8 Plus",
	},
	// iPhone X
	"D22AP": {
		Width:  "375",
		Height: "812",
		Model:  "iPhone X",
	},
	// iPhone X
	"D221AP": {
		Width:  "375",
		Height: "812",
		Model:  "iPhone X",
	},
	// iPhone XR
	"N841AP": {
		Width:  "414",
		Height: "896",
		Model:  "iPhone XR",
	},
	// iPhone XS
	"D321AP": {
		Width:  "375",
		Height: "812",
		Model:  "iPhone XS",
	},
	// iPhone XS Max
	"D331pAP": {
		Width:  "414",
		Height: "896",
		Model:  "iPhone XS Max",
	},
	// iPhone XS Max
	"D331AP": {
		Width:  "414",
		Height: "896",
		Model:  "iPhone XS Max",
	},
	// iPhone 11
	"N104AP": {
		Width:  "414",
		Height: "896",
		Model:  "iPhone 11",
	},
	// iPhone 11 Pro
	"D421AP": {
		Width:  "375",
		Height: "812",
		Model:  "iPhone 11 Pro",
	},
	// iPhone 11 Pro Max
	"D431AP": {
		Width:  "414",
		Height: "896",
		Model:  "iPhone 11 Pro Max",
	},
	// iPhone 12 Mini
	"D52gAP": {
		Width:  "360",
		Height: "780",
		Model:  "iPhone 12 Mini",
	},
	// iPhone 12
	"D53gAP": {
		Width:  "390",
		Height: "844",
		Model:  "iPhone 12",
	},
	// iPhone 12 Pro
	"D53pAP": {
		Width:  "390",
		Height: "844",
		Model:  "iPhone 12 Pro",
	},
	// iPhone 12 Pro Max
	"D54pAP": {
		Width:  "428",
		Height: "926",
		Model:  "iPhone 12 Pro Max",
	},
	// iPhone 13 Mini
	"D16AP": {
		Width:  "360",
		Height: "780",
		Model:  "iPhone 13 Mini",
	},
	// iPhone 13
	"D17AP": {
		Width:  "390",
		Height: "844",
		Model:  "iPhone 13",
	},
	// iPhone 13 Pro
	"D63AP": {
		Width:  "390",
		Height: "844",
		Model:  "iPhone 13 Pro",
	},
	// iPhone 13 Pro Max
	"D64AP": {
		Width:  "428",
		Height: "926",
		Model:  "iPhone 13 Pro Max",
	},
	// iPhone SE (3rd gen)
	"D49AP": {
		Width:  "375",
		Height: "667",
		Model:  "iPhone SE (3rd gen)",
	},
	// iPhone 14
	"D27AP": {
		Width:  "390",
		Height: "844",
		Model:  "iPhone 14",
	},
	// iPhone 14 Plus
	"D28AP": {
		Width:  "428",
		Height: "926",
		Model:  "iPhone 14 Plus",
	},
	// iPhone 14 Pro
	"D73AP": {
		Width:  "393",
		Height: "852",
		Model:  "iPhone 14 Pro",
	},
	// iPhone 14 Pro Max
	"D74AP": {
		Width:  "430",
		Height: "932",
		Model:  "iPhone 14 Pro Max",
	},
}

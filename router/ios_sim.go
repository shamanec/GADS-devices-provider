package router

import (
	"github.com/gin-gonic/gin"
	"github.com/shamanec/GADS-devices-provider/ios_sim"
	"net/http"
)

func BootSim(c *gin.Context) {
	udid := c.Param("udid")

	bootedSims, err := ios_sim.GetBootedSims()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if len(bootedSims) > 4 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Maximum number of booted simulators reached",
		})
		return
	}

	err = ios_sim.BootSim(udid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Simulator booted successfully",
	})
}

func ShutdownSim(c *gin.Context) {
	udid := c.Param("udid")
	err := ios_sim.ShutdownSim(udid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Simulator shutdown successfully",
	})
}

func GetAvailableSims(c *gin.Context) {
	sims, err := ios_sim.GetAvailableSims()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sims": sims,
	})
}

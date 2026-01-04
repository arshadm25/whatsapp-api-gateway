package main

import (
	"encoding/json"
	"log"
	"whatsapp-gateway/internal/automation"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"

	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()
	database.InitGorm(cfg)
	db := database.GormDB

	log.Println("Migrating JSON flows to relational tables...")

	var flows []struct {
		ID        string
		GraphData string
	}

	// Use raw SQL to get the column we just removed from the GORM model
	if err := db.Raw("SELECT id, graph_data FROM flows WHERE graph_data IS NOT NULL AND graph_data != ''").Scan(&flows).Error; err != nil {
		log.Fatalf("Error fetching legacy flows: %v", err)
	}

	for _, f := range flows {
		log.Printf("Processing flow: %s", f.ID)

		var graph automation.FlowGraphData
		if err := json.Unmarshal([]byte(f.GraphData), &graph); err != nil {
			log.Printf("Error unmarshaling graph data for %s: %v", f.ID, err)
			continue
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			// Insert nodes
			for _, n := range graph.Nodes {
				dataJSON, _ := json.Marshal(n.Data)
				node := models.FlowNode{
					FlowID:    f.ID,
					NodeID:    n.ID,
					Type:      n.Type,
					PositionX: n.Position["x"],
					PositionY: n.Position["y"],
					Data:      string(dataJSON),
				}
				if err := tx.FirstOrCreate(&node, models.FlowNode{FlowID: f.ID, NodeID: n.ID}).Error; err != nil {
					return err
				}
			}

			// Insert edges
			for _, e := range graph.Edges {
				edge := models.FlowEdge{
					FlowID:       f.ID,
					EdgeID:       e.ID,
					Source:       e.Source,
					Target:       e.Target,
					SourceHandle: e.SourceHandle,
				}
				if err := tx.FirstOrCreate(&edge, models.FlowEdge{FlowID: f.ID, EdgeID: e.ID}).Error; err != nil {
					return err
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("Error migrating flow %s: %v", f.ID, err)
		} else {
			log.Printf("Successfully migrated flow %s", f.ID)
		}
	}

	log.Println("Done!")
}

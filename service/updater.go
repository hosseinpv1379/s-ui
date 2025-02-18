package service

import (
    "s-ui/database"
    "s-ui/logger"
    "s-ui/database/model"
    "time"
    "sync"
    "encoding/json"
)

type UpdaterService struct {
    clientService  *ClientService
    lastUpdate    int64
    running       bool
    mutex         sync.Mutex
}

func NewUpdaterService(clientService *ClientService) *UpdaterService {
    return &UpdaterService{
        clientService: clientService,
        lastUpdate: time.Now().Unix(),
        running:    false,
    }
}

func (s *UpdaterService) Start() error {
    s.mutex.Lock()
    if s.running {
        s.mutex.Unlock()
        return nil
    }
    s.running = true
    s.mutex.Unlock()

    logger.Info("UpdaterService started")

    go func() {
        for s.running {
            if err := s.checkAndUpdate(); err != nil {
                logger.Warning("updater error: ", err)
            }
            time.Sleep(5 * time.Second)
        }
    }()

    return nil
}

func (s *UpdaterService) Stop() error {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    s.running = false
    logger.Info("UpdaterService stopped")
    return nil
}

func (s *UpdaterService) checkAndUpdate() error {
    db := database.GetDB()
    tx := db.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // بررسی تغییرات جدید
    var clients []model.Client
    err := tx.Model(&model.Client{}).Where("created_at > ?", s.lastUpdate).Find(&clients).Error
    if err != nil {
        tx.Rollback()
        return err
    }

    if len(clients) > 0 {
        logger.Info("Found ", len(clients), " new clients to update")

        for _, client := range clients {
            var inboundIds []uint
            err = json.Unmarshal(client.Inbounds, &inboundIds)
            if err != nil {
                tx.Rollback()
                return err
            }

            // استفاده از متد ClientService برای آپدیت
            hostname := "localhost" // یا دریافت از کانفیگ
            err = s.clientService.UpdateLinksByInboundChange(tx, inboundIds, hostname)
            if err != nil {
                tx.Rollback()
                return err
            }
        }

        // ذخیره تغییرات
        err = tx.Commit().Error
        if err != nil {
            return err
        }

        logger.Info("Successfully updated ", len(clients), " clients")
    }

    s.lastUpdate = time.Now().Unix()
    return nil
}
package entity

import (
	"time"
)

// Tenants is an array of Tenant
type Tenants []*Tenant

// Tenant defines the tenant
type Tenant struct {
	Uid        string    `json:"uid" bson:"uid"`
	Zone       string    `json:"zone" bson:"zone"`
	Partition  string    `json:"partition" bson:"partition"`
	Priority   int32     `json:"priority" bson:"priority"`
	Name       string    `json:"name" bson:"name"`
	Status     string    `json:"status" bson:"status"`
	CreatedAt  time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt" bson:"updatedAt"`
	LastActive time.Time `json:"lastActive" bson:"lastActive"`

	ResourceQuota ResourceQuota `bson:"-" json:"resourceQuota"`
}

func (t *Tenant) Fields() []interface{} {
	return []interface{}{
		&t.Uid, &t.Zone, &t.Priority, &t.Partition, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.LastActive,
	}
}

func (t *Tenant) FieldsWithQuota() []interface{} {
	dst := []interface{}{
		&t.Uid, &t.Zone, &t.Priority, &t.Partition, &t.Name, &t.Status, &t.CreatedAt, &t.UpdatedAt, &t.LastActive,
	}
	dst = append(dst, t.ResourceQuota.Fields()...)
	return dst
}

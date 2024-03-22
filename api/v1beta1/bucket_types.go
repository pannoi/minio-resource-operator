package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BucketSpec struct {
	Name          string         `json:"name"`
	ObjectLocking ObjectLocking  `json:"objectLocking,omitempty"`
	Versioning    VersioningSpec `json:"versioning,omitempty"`
}

type ObjectLocking struct {
	Enabled   bool   `json:"enabled,omitempty"`
	Mode      string `json:"mode"`
	Retention int    `json:"retention"`
}

type VersioningSpec struct {
	Enabled bool `json:"enabled,omitempty"`
}

type BucketStatus struct {
	Conditions []metav1.Condition `json:"conditions"`
}

type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}

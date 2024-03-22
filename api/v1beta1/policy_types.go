package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicySpec struct {
	Name      string `json:"name"`
	Statement string `json:"statement"`
}

// TODO: Insert PolicyStatus
type PolicyStatus struct {
}

type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec"`
	Status PolicyStatus `json:"status,omitempty"`
}

type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TeamspeakSpec struct {
	Version string `json:"version"`
}

type TeamspeakStatus struct {
	Address string `json:"address"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Teamspeak struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TeamspeakSpec   `json:"spec,omitempty"`
	Status TeamspeakStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TeamspeakList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Teamspeak `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Teamspeak{}, &TeamspeakList{})
}

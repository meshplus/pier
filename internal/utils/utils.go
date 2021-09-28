package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseServicePair(servicePair string) (string, string, error) {
	splits := strings.Split(servicePair, "-")
	if len(splits) != 2 {
		return "", "", fmt.Errorf("invalid service pair ID: %s", servicePair)
	}

	return splits[0], splits[1], nil
}

func ParseFullServiceID(serviceID string) (string, string, string, error) {
	splits := strings.Split(serviceID, ":")
	if len(splits) != 3 {
		return "", "", "", fmt.Errorf("invalid service ID: %s", serviceID)
	}

	return splits[0], splits[1], splits[2], nil
}

func ParseIBTPID(id string) (string, string, uint64, error) {
	splits := strings.Split(id, "-")
	if len(splits) != 3 {
		return "", "", 0, fmt.Errorf("invalid IBTP ID: %s", id)
	}

	index, err := strconv.Atoi(splits[2])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid IBTP ID: %s", id)
	}

	return splits[0], splits[1], uint64(index), nil
}

func GetSrcDstBitXHubID(id string, isReq bool) (string, string, error) {
	splis := strings.Split(id, "-")
	if len(splis) != 3 {
		return "", "", fmt.Errorf("invalid ibtp id %s", id)
	}

	bxhID0, _, _, err := ParseFullServiceID(splis[0])
	if err != nil {
		return "", "", err
	}

	bxhID1, _, _, err := ParseFullServiceID(splis[1])
	if err != nil {
		return "", "", err
	}

	if isReq {
		return bxhID0, bxhID1, nil
	}
	return bxhID1, bxhID0, nil
}

func GetPierID(serviceID string) (string, error) {
	_, appchainId, _, err := ParseFullServiceID(serviceID)
	if err != nil {
		return "", err
	}
	return appchainId, nil
}

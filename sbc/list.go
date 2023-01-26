package sbc

import (
	"fmt"

	"github.com/ZeljkoBenovic/tsbc/sbc/types"
)

func (s *sbc) List() ([]types.Sbc, error) {
	var (
		sbcsInfo []types.Sbc
		sbcInfo  types.Sbc
		err      error
	)

	allSbcNames, err := s.db.GetAllFqdnNames()
	if err != nil {
		return nil, fmt.Errorf("could not get SBC names: %w", err)
	}

	for _, sbcName := range allSbcNames {
		sbcInfo, err = s.db.GetSBCParameters(s.db.GetSBCIdFromFqdn(sbcName))
		if err != nil {
			s.logger.Error("Could not get sbc information", "fqdn", sbcName, "err", err)
		}

		sbcsInfo = append(sbcsInfo, sbcInfo)
	}

	return sbcsInfo, nil
}

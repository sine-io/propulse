package communitymarket

import (
	"math"
	"testing"
	"time"
)

func TestSnapshotDataNormalizeProfile(t *testing.T) {
	data := SnapshotData{
		ProvinceCode:              " 120000 ",
		ProvinceName:              " 天津市 ",
		PropertyType:              " 普通住宅 ",
		PropertyTags:              []string{" 商品房 ", "私产", "商品房", " "},
		Developer:                 " 天津耀华投资发展有限公司 ",
		ClosedManagement:          " 是 ",
		PropertyManagementCompany: " 天津碧桂园物业有限公司 ",
		ManCarSeparation:          " 否 ",
	}.Normalize()

	if data.ProvinceCode != "120000" || data.ProvinceName != "天津市" || data.PropertyType != "普通住宅" {
		t.Fatalf("normalized profile identity = %#v", data)
	}
	if len(data.PropertyTags) != 2 || data.PropertyTags[0] != "商品房" || data.PropertyTags[1] != "私产" {
		t.Fatalf("normalized property tags = %#v", data.PropertyTags)
	}
	if data.Developer != "天津耀华投资发展有限公司" || data.ClosedManagement != "是" || data.ManCarSeparation != "否" {
		t.Fatalf("normalized profile text = %#v", data)
	}
}

func TestSnapshotDataValidateAllowsMissingProfileAndChineseEnumValues(t *testing.T) {
	data := validSnapshotData()
	if violations := data.Validate(time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)); len(violations) != 0 {
		t.Fatalf("missing optional profile violations = %#v", violations)
	}

	buildingCount, buildingYear, households, parking := 11, 2012, 1089, 1550
	plotRatio, greenArea, greeningRate := 1.8, 14271.0, 40.0
	data.BuildingCount = &buildingCount
	data.BuildingYear = &buildingYear
	data.HouseholdCount = &households
	data.FixedParkingSpaces = &parking
	data.PlotRatio = &plotRatio
	data.GreenAreaSQM = &greenArea
	data.GreeningRatePercent = &greeningRate
	data.ClosedManagement = "是"
	data.ManCarSeparation = "否"
	if violations := data.Validate(time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)); len(violations) != 0 {
		t.Fatalf("complete profile violations = %#v", violations)
	}
}

func TestSnapshotDataValidateRejectsInvalidProfileRangesAndEnums(t *testing.T) {
	data := validSnapshotData()
	negative, futureYear := -1, 2027
	invalidRatio, invalidRate, nonFinite := 101.0, -0.1, math.Inf(1)
	data.BuildingCount = &negative
	data.BuildingYear = &futureYear
	data.HouseholdCount = &negative
	data.FixedParkingSpaces = &negative
	data.PlotRatio = &invalidRatio
	data.GreenAreaSQM = &nonFinite
	data.GreeningRatePercent = &invalidRate
	data.ClosedManagement = "未知"
	data.ManCarSeparation = "有"

	violations := data.Validate(time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC))
	for _, field := range []string{
		"buildingCount", "buildingYear", "householdCount", "fixedParkingSpaces", "plotRatio",
		"greenAreaSqm", "greeningRatePercent", "closedManagement", "manCarSeparation",
	} {
		if !hasViolation(violations, field) {
			t.Fatalf("violations = %#v, want field %q", violations, field)
		}
	}
}

func validSnapshotData() SnapshotData {
	return SnapshotData{
		SourceCommunityID: "source-community",
		CommunityName:     "鸣泉花园",
		CityCode:          "120100",
		CityName:          "天津市",
		DistrictCode:      "120111",
		DistrictName:      "西青区",
		BlockCode:         "block",
		BlockName:         "梅江南",
		Latitude:          39,
		Longitude:         117,
	}
}

func hasViolation(violations []Violation, field string) bool {
	for _, violation := range violations {
		if violation.Field == field {
			return true
		}
	}
	return false
}

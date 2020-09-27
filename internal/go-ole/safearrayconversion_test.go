// +build windows

package ole

import (
	"fmt"
	"strings"
	"testing"
)

// This tests more than one function. It tests all of the functions needed in order to retrieve an
// SafeArray populated with Strings.
func TestSafeArrayConversionString(t *testing.T) {
	CoInitialize(0)
	defer CoUninitialize()

	clsid, err := CLSIDFromProgID("QBXMLRP2.RequestProcessor.1")
	if err != nil {
		if err.(*OleError).Code() == CO_E_CLASSSTRING {
			return
		}
		t.Log(err)
		t.FailNow()
	}

	unknown, err := CreateInstance(clsid, IID_IUnknown)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	defer unknown.Release()

	dispatch, err := unknown.QueryInterface(IID_IDispatch)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var dispid []int32
	dispid, err = dispatch.GetIDsOfName([]string{"OpenConnection2"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	var result *VARIANT
	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, "", "Test Application 1", 1)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	dispid, err = dispatch.GetIDsOfName([]string{"BeginSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	result, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, "", 2)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	ticket := result.ToString()

	dispid, err = dispatch.GetIDsOfName([]string{"QBXMLVersionsForSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	result, err = dispatch.Invoke(dispid[0], DISPATCH_PROPERTYGET, ticket)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	// Where the real tests begin.
	conversion := result.ToArray()

	totalElements, _ := conversion.TotalElements(0)
	if totalElements != 13 {
		t.Log(fmt.Sprintf("%d total elements does not equal 13\n", totalElements))
		t.Fail()
	}

	versions := conversion.ToStringArray()
	if len(versions) != 13 {
		t.Log(fmt.Sprintf("%s\n", strings.Join(versions, ", ")))
		t.Fail()
	}

	conversion.Release()

	dispid, err = dispatch.GetIDsOfName([]string{"EndSession"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD, ticket)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	dispid, err = dispatch.GetIDsOfName([]string{"CloseConnection"})
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	_, err = dispatch.Invoke(dispid[0], DISPATCH_METHOD)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
}

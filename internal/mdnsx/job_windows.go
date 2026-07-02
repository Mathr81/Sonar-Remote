package mdnsx

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// tieToOurLifetime puts the child process in a Job Object flagged
// KILL_ON_JOB_CLOSE. When our process exits — cleanly, crash, or hard kill —
// the OS closes the job handle and terminates the child, so stray dns-sd
// registrations never pile up. The job handle is intentionally kept open for
// the whole lifetime of this process.
func tieToOurLifetime(p *os.Process) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return
	}
	h, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(p.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(job, h)
	// Deliberately do NOT close `job`: it must stay open until we exit.
}

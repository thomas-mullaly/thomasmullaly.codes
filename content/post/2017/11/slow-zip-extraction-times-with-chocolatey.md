---
title: "Resolving Slow Archive Extraction Times with Chocolatey"
date: 2017-11-19
draft: false
author: "Thomas Mullaly"
tags: ["windows", "chocolatey", "windows defender"]
categories: ["pro-tip"]
---

## The Problem

Over the weekend I kept getting chocolately timeout errors when I was trying to upgrade my [golang](https://golang.org) installation to the latest release (1.9.2).

![Chocolatey timeout](/images/post/2017/11/timeout.png)

This was puzzling because, as the error message describes, Chocolatey's default timeout period is 2700 seconds (**45 minutes**) long. It seems Chocolatey was attempting to extract the golang archive but got stuck trying to do so. I'm running an SSD on my desktop at home so it should have been able to make short work of that extraction. Thinking back on previous golang upgrade experiences, I've always noticed it took awhile (20-30 minutes) to extract the zip file but up until now I had never experienced a timeout.

## The Investigation

After my 3rd failed upgrade attempt, I decided to dig deeper and figure out what was going on. My first step was to fire up Task Manager and see if anything stood out in the "Processes" tab. The only thing I noticed, which was noteworthy, was that "Antimalware Service Executable" (Windows Defender) process was using 10-12% of the CPU.

![Task Manager - Windows Defender Usage](/images/post/2017/11/task-manager.png)

This seemed a little strange but maybe it was just running a scan at the same time I was looking at Task Manager? Unfortunately Task Manager doesn't provide much granularity as to _what_ a process is doing, so I needed another tool which could provide that insight. Luckily Microsoft has [Process Monitor](https://docs.microsoft.com/en-us/sysinternals/downloads/procmon) (Procmon) as a part of their "Sysinternals" collection. If you've never used Process Monitor before, it's like Task Manager but on steroids. It shows (almost) everything a process is doing on your machine (ie. Registry access, file access, network activity, threading/processes info). The amount of information Procmon collects is overwhelming if you don't filter down the results it displays. Since I already had a general idea of what I was looking for, I added filters to Procmon to only display filesystem events for `choco.exe` and `MsMpEng.exe` (this is the exe file Windows Defender runs when scanning a file for malware).

![Process Monitor - choco.exe and MsMpEng.exe](/images/post/2017/11/procmon.png)

Well that's interesting. Seems Windows Defender is trying to scan Chocolatey's log file everytime Chocolatey writes to it. This makes sense as most Anti-virus software will hook into filesystem events so that it can proactively scan files to detect malware/viruses. Unfortunately for me, Chocolatey writes verbose logs to the `chocolateysummary.log` file. Specifically, it writes out every file/folder that's being extracted from the zip file. In the case of golang that's nearly 8,000 files!

## The Solution

Now that I had a pretty good idea what was happening the solution ended up being a super easy fix.

In "Windows Defender Security Center" I clicked on "Virus & threat proctection" then "Virus & threat protection settings". If you scroll down there's an "Exclusions" section where you can add/remove exclusions. Exclusions will prevent Windows Defender from proactively scanning certain folders/files on your computer. In my case I added the `C:\ProgramData\Chocolatey\logs` folder to the list of exclusions. Within 30 seconds of me adding this exculsion, Chocolatey was able to completely extract the archive and finish the upgrade.

To verify my change, I completely uninstalled golang and tried a fresh install (with the exclusion rule configured) and was able to do a complete golang install within a minute or two. I tried to fresh install again with the Windows Defender exclusion removed and chocolatey immediately got hung up trying to extract the golang archive.

While this helped me resolve my golang issue, this fix is probably widely applicable to any chocolatey package which installs via a zip file.

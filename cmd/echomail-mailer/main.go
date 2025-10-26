package main

import (
	"echomail/envelope"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func fail(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, msg+": %v\n", err)
		os.Exit(1)
	}
}

func receiveMail(root string, msg envelope.EchomailMessage) error {
	// By this point, we definitely have a group on the message.

	groupDir := envelope.GroupDir(msg.Group)
	msgPath := filepath.Join(root, groupDir, msg.MsgId)
	err := os.MkdirAll(msgPath, 0777)
	if err != nil {
		return err
	}

	os.WriteFile(filepath.Join(msgPath, "index.gmi"), *msg.Message, 0666)
	for fn, fc := range msg.Attachments {
		attachPath := filepath.Join(msgPath, fn)
		// Technically, redundant, since we check for that up above.
		if !strings.HasPrefix(attachPath, msgPath) {
			return fmt.Errorf("path traversal in attachment: %s", fn)
		}
		_, err := os.Stat(attachPath)
		if err == nil {
			return fmt.Errorf("attempt to overwrite attachment: %s", attachPath)
		}
		err = os.WriteFile(attachPath, *fc, 0666)
		if err != nil {
			return fmt.Errorf("failed to save attachment file: %s", attachPath)
		}
	}

	return nil
}

func sendMail(root string, body []byte, noSave bool, attachments []string) error {
	// First, build the envelope...
	en := envelope.Envelope{}
	en["."] = body
	if len(attachments) > 0 {
		for _, fn := range attachments {
			newFn := filepath.Base(fn)
			if newFn == "." {
				return fmt.Errorf("illegal attachment name")
			}
			data, err := os.ReadFile(fn)
			if err != nil {
				return fmt.Errorf("error reading attachment: %w", err)
			}
			en[newFn] = data
		}
	}
	err := en.Verify()
	if err != nil {
		return err
	}
	// Unless told otherwise, save it to the root as usual.
	if !noSave {
		msg, err := en.Parse()
		if err != nil {
			return err
		}
		err = receiveMail(root, msg)
		if err != nil {
			return err
		}
	}
	// Dump the envelope to standard output.
	data, err := en.Write()
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func addMailRoot(fs *flag.FlagSet) *string {
	return fs.String("root", "", "Root of the received echomail tree.")
}

func testMailRoot(mailRoot string) {
	info, err := os.Stat(mailRoot)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "mail root must be a directory: %v\n", err)
		os.Exit(1)
	}
}

func printGlobalUsage() {
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  %s <command> [options]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  receive   Receive a message. Use in NNCP exec.\n")
	fmt.Fprintf(os.Stderr, "  send      Send a message. Packages a message into an envelope to be piped into an NNCP exec.\n")
	fmt.Fprintf(os.Stderr, "\nUse \"%s <command> -h\" for more information about a command.\n", os.Args[0])
	os.Exit(1)
}

func main() {

	if len(os.Args) < 2 {
		printGlobalUsage()
	}

	switch os.Args[1] {
	case "receive":

		receiveCmd := flag.NewFlagSet("receive", flag.ExitOnError)
		receiveCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "\nUsage: %s receive [options] <args>\n", os.Args[0])
			receiveCmd.PrintDefaults()
			fmt.Fprintf(os.Stderr, "\nA message envelope is expected at standard input.\n")
		}

		mailRoot := addMailRoot(receiveCmd)
		noVerifySender := receiveCmd.Bool("noorigin", false, "Do not verify message sender against NNCP_ORIGIN")
		receiveCmd.Parse(os.Args[2:])

		testMailRoot(*mailRoot)

		data, err := io.ReadAll(os.Stdin)
		fail(err, "error reading stdin")
		env, err := envelope.Read(data)
		fail(err, "error unwrapping")
		mail, err := env.Parse()
		fail(err, "error parsing message")

		if !*noVerifySender {
			sender, isSet := os.LookupEnv("NNCP_ORIGIN")
			if isSet {
				if sender != mail.Sender {
					fmt.Fprintf(os.Stderr, "sender spoof detected")
					os.Exit(1)
				}
			} else {
				fmt.Fprintf(os.Stderr, "NNCP_ORIGIN environment variable is not set. If your version of NNCP is too old to supply this, you may want to use -noorigin until you can update.")
				os.Exit(1)
			}
		}

		err = receiveMail(*mailRoot, mail)
		fail(err, "error saving message")
		os.Exit(0)
	case "send":
		sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
		mailRoot := addMailRoot(sendCmd)
		noSave := sendCmd.Bool("nosave", false, "Do not save the message being sent to our echomail tree")
		sendCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: %s send [options] <args>\n\n", os.Args[0])
			sendCmd.PrintDefaults()
			fmt.Fprintf(os.Stderr, "\nThe message text is to be supplied on standard input, as gemtext.\n"+
				"Any attachments are to be given as command line arguments after all the options.\n"+
				"The message envelope will be sent to standard output, to be piped to nncp-exec.\n\n")
		}
		sendCmd.Parse(os.Args[2:])

		testMailRoot(*mailRoot)

		data, err := io.ReadAll(os.Stdin)
		fail(err, "error reading stdin")
		err = sendMail(*mailRoot, data, *noSave, sendCmd.Args())
		fail(err, "error sending message")
	default:
		printGlobalUsage()
	}
}

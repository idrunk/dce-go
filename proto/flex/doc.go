/*
Package flex

Package bit format routable protocol struct
Protocol format:

	 0               1               .               .               .
	 0 1 2 3 4 5 6 7 0 . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . .
	+-+-+-+-+-+-+-+-+- - - - - - - - -  - - - - - - - - - - - - - - - - - - - - - - |
	|I|P|S|C|M|L|N|R| LEN of| LEN of| LEN of| LEN of|  ID   |  CODE |NumPath| same  |
	|D|A|I|O|S|O|P|S| Path  | Sid   | Msg   |  Body |FlexNum|FlexNum|FlexNum| order |
	|E|T|D|D|G|A|A|V|FlexNum|FlexNum|FlexNum|FlexNum| HEAD  | HEAD  | HEAD  |FlexNum|
	|N|H| |E| |D|T| | HEAD  | HEAD  | HEAD  | HEAD  |       |       |       | BODY  |
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ - - - - - - - - - - - - - - - - - - - - - - - |
	|      |     |     |                                                            |
	| Path | Sid | Msg |                       Body Data ...                        |
	|      |     |     |                                                            |
	+-+-------------+-+-------------+-+-------------+-------------------------------+

	First byte is bit flags, or be an empty package if not set.
	IDEN: with an request Id
	PATH: with a request Path
	SID: with a session Id
	CODE: with an error Code
	MSG: with error messages
	LOAD: with payload data
	NPAT: with a number Path

	LEN of xxx FlexNum HEAD: The FlexNum HEAD of xxx's length
	xxx FlexNum HEAD: The FlexNum HEAD of xxx number
	same order FlexNum BODY: The FlexNum BODYs with same order to the heads
	PATH part: the api Path. (No this part if the NPAT is set)
	SID part: the session Id
	MESG part: the error Msg
	Payload part: the payload data

Definition of flexible length sequence numbers:

	 0               1               2               3               4               5               6
	 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-
	|0|S|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-
	|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-
	|1|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-
	|1|1|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-
	|1|1|1|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-
	|1|1|1|1|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-
	|1|1|1|1|1|1|0|S|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B
	|1|1|1|1|1|1|1|0|S|      Reserve for int128, max 1+8+8=17 bytes.                             S: negative sign
	|1|1|1|1|1|1|1|1|        Reserve for other situations.                                       B: binary number
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-
	 7               8               9               10              11              12              13
	 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0 7 6 5 4 3 2 1 0
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|B|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/
package flex

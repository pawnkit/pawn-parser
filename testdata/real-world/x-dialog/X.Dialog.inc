/*
* includes X.Dialog
* to make it easier to create dialogue
* version X-Beta 0.1a
* created by X-Clubs
*/

#if defined _X_DIALOG_INC
    #endinput
#endif
#define _X_DIALOG_INC

// Message Dialog
stock XDialog(playerid,
    const title[],
    const text1[],
    const text2[] = "",
    const text3[] = ""
)
{
    new string[1024];

    if(strlen(text2) && strlen(text3))
    {
        format(string, sizeof(string),
            "%s\n%s\n%s",
            text1,
            text2,
            text3
        );
    }
    else if(strlen(text2))
    {
        format(string, sizeof(string),
            "%s\n%s",
            text1,
            text2
        );
    }
    else
    {
        format(string, sizeof(string),
            "%s",
            text1
        );
    }

    return ShowPlayerDialog(
        playerid,
        1000,
        DIALOG_STYLE_MSGBOX,
        title,
        string,
        "OK",
        ""
    );
}

// Input Dialog
stock XInput(playerid,
    const title[],
    const text1[],
    const text2[] = ""
)
{
    new string[1024];

    if(strlen(text2))
    {
        format(string, sizeof(string),
            "%s\n%s",
            text1,
            text2
        );
    }
    else
    {
        format(string, sizeof(string),
            "%s",
            text1
        );
    }

    return ShowPlayerDialog(
        playerid,
        1001,
        DIALOG_STYLE_INPUT,
        title,
        string,
        "Submit",
        "Cancel"
    );
}

// List Dialog
stock XList(playerid,
    const title[],
    const list1[],
    const list2[] = ""
)
{
    new string[1024];

    if(strlen(list2))
    {
        format(string, sizeof(string),
            "%s\n%s",
            list1,
            list2
        );
    }
    else
    {
        format(string, sizeof(string),
            "%s",
            list1
        );
    }

    return ShowPlayerDialog(
        playerid,
        1002,
        DIALOG_STYLE_LIST,
        title,
        string,
        "select",
        "cancel"
    );
}

from tkinter import *
from tkinter import ttk
from tkinter.filedialog import asksaveasfilename, askopenfilename
import re
import os
import zlib, pickle
from icon import icon
from multiprocessing import freeze_support, Process
import ctypes

try:
    # Windows 8.1 and later
    ctypes.windll.shcore.SetProcessDpiAwareness(2) 
except AttributeError:
    try:
        # Windows 7
        ctypes.windll.user32.SetProcessDPIAware()
    except AttributeError:
        pass

def to_tk(text, ind):
    lines = 1
    chars = 0
    for char in text:
        if ind<0:
            break
        if char=="\n":
            lines+=1
            chars = 0
        else:
            chars+=1
        ind-=1
    return f"{lines}.{chars-1}"

def remove_comment(line):
    if "@" in line:
        return line.split("@")[0]
    else:
        return line

def clean(text):
    while text and text[0]==" ":
        text = text[1:]
    while text and text[-1]==" ":
        text = text[:-1]
    return text

class Search(Toplevel):
    def __init__(self, master):
        super().__init__(master)
        self.master = master
        self.mode = "simple"
        self.found = 0
        self.withdraw()
        self.minsize(400, 30)
        self.resizable(True, False)

        self.var = StringVar()
        self.entry = ttk.Entry(self, textvariable = self.var, font = ("Calibri", 14))
        self.search_button = ttk.Button(self, text = "Search", command = self.search)
        self.mode_button = ttk.Button(self, text = f"Mode: {self.mode}", command = self.switch)
        self.entry.pack(fill = X, expand = 1, side = LEFT)
        self.search_button.pack(fill = Y, side = RIGHT)
        self.mode_button.pack(fill = Y, side = RIGHT)
        self.protocol("WM_DELETE_WINDOW", self.on_closing)
        self.bind("<Return>", self.search)
        self.master.t_editor.bind("<FocusIn>", lambda e: self.master.t_editor.tag_remove("sel2", "1.0", END))
        self.master.t_editor.tag_config("sel2", foreground = "#ffffff", background = "#0078D7")

    def on_closing(self, event = None):
        self.withdraw()
        self.master.t_editor.tag_remove("sel2", "1.0", END)

    def search(self, event = None):
        self.master.t_editor.tag_remove("sel2", "1.0", END)
        if self.mode=="simple":
            self.found = self.master.t_editor.get("1.0","end-1c").find(self.var.get(), self.found)
            if self.found==-1:
                self.found = 0
                self.found = self.master.t_editor.get("1.0","end-1c").find(self.var.get(), self.found)
            if self.found!=-1:
                start = to_tk(self.master.t_editor.get("1.0","end-1c"), self.found)
                end = f"{start}+{len(self.var.get())}c"
                self.master.t_editor.tag_remove("sel2", "1.0", END)
                #self.master.t_editor.focus_set()
                self.master.t_editor.mark_set("insert", end)
                self.master.t_editor.tag_add("sel2", start, end)
                self.master.t_editor.see(start)
                self.found+=len(self.var.get())
        else:
            pattern = self.var.get()
            matches = list(re.finditer(pattern, self.master.t_editor.get("1.0","end-1c")))
            if len(matches)>0:
                self.master.t_editor.tag_remove("sel2", "1.0", END)
                #self.master.t_editor.focus_set()
                obj = matches[self.found]
                self.found = (self.found+1)%len(matches)
                start = to_tk(self.master.t_editor.get("1.0","end-1c"), obj.start(0))
                end = to_tk(self.master.t_editor.get("1.0","end-1c"), obj.end(0))
                if self.master.t_editor.get(start, end).endswith("\n"):
                    end+="-1c"
                self.master.t_editor.tag_add("sel2", start, end)
                self.master.t_editor.mark_set("insert", end)
                self.master.t_editor.see(start)
                self.master.t_editor.tag_add("sel", start, end)

    def switch(self):
        self.mode = ["simple", "regex"][(["simple", "regex"].index(self.mode)+1)%2]
        self.found = 0
        self.mode_button.config(text = f"Mode: {self.mode}")

def worker(command):
    os.system(command)

class GUI(Tk):
    def __init__(self, master = None):
        if master:
            super().__init__(master)
        else:
            super().__init__()
        try:
            pi = PhotoImage(data=icon)
            self.wm_iconphoto(True, pi)
        except Exception:
            pass

        self.depth = 0
        self.interpreter = ""
        self.user_funs = []
        self.savers = ["<space>", "<KeyRelease-Return>", "<(>", "<)>", "<,>", "<KeyRelease-BackSpace>"] if os.name=="nt" else ["<space>", "<KeyRelease-Return>", "(", ")", ",", "<KeyRelease-BackSpace>"]
        self.history = zlib.compress(pickle.dumps([("","1.0")]), 9)
        self.cancelled = []
        self.his_focus = 0
        self.wid = 1# t_lines width
        font = ("Courier", 14)
        self.title("Miniscule Minimum Editor")
        self.t_lines = Text(self, autoseparators=True, undo = True, relief = FLAT, width = self.wid, font = font)
        self.t_editor = Text(self, autoseparators=True, undo = True, relief = FLAT, font = font, wrap = NONE)
        self.scrollbar = ttk.Scrollbar(self, command=self.multiple_yview)
        self.t_lines['yscrollcommand'] = self.scrollbar.set
        self.t_editor['yscrollcommand'] = self.scrollbar.set
        self.grid_columnconfigure(1, weight = 1)
        self.grid_rowconfigure(0, weight = 1)

        self.t_lines.grid(row = 0, column = 0, sticky = "wens")
        self.t_editor.grid(row = 0, column = 1, sticky = "wens", padx = (20,0))
        self.scrollbar.grid(row = 0, column = 2, sticky = "wens")

        self.searcher = Search(self)

        self.t_editor.bind("<Key>", lambda e: (self.input_manager(e), self.scroll_left(e), self.his_update(e)) )
        #self.t_editor.bind("<Key>", self.scroll_left)
        if os.name=="nt":
            self.t_editor.bind('<MouseWheel>', self.advanced_scroll2)
            self.t_lines.bind('<MouseWheel>', self.advanced_scroll1)
        else:
            self.t_editor.bind('<Button-4>', self.scroll_left)
            self.t_editor.bind('<Button-5>', self.scroll_left)
        if os.name=="nt":
            self.bind("<Control-Shift-{>", self.remove_indent)
            self.bind("<Control-Shift-}>", self.add_indent)
        else:
            self.bind("<Control-Shift-O>", self.remove_indent)
            self.bind("<Control-Shift-P>", self.add_indent)
        self.bind("<Control-Shift-S>", self.save_as)
        self.bind("<Control-s>", self.save)
        self.bind("<Control-o>", self.open)
        self.bind("<Control-z>", self.his_left)
        self.bind("<Control-Shift-Z>", self.his_right)
        self.bind("<Control-f>", self.search)
        self.bind("<F5>", self.run)
        self.bind("<Control-n>", self.new_window)
        for saver in self.savers:
            self.t_editor.bind(saver, self.his_save)
        
        self.indent = 4
        self.fname = ""
        self.pfuns = ["mark","proc","print","out","prec","return","sleep", "env", "except","quit"]
        self.funs = ["sort","format", "join", "len", "list", "append", "pop", "convert", "sub", "type", "system", "input", "has", "range", "index", "run", "read", "write", "array", "itc", "cti", "rand", "checkpoint", "help", "exec", "pair", "keys", "ternary", "id", "value"]
        #self.funs = [f"{f} " for f in self.funs]
        self.synt = ["or","not","and","true","false","if", "while", "pool", "<-", "->", "set", "for", "else", "func", "--", "source", "library", "error", "switch", "case", "process", "global", "repeat"]
        self.exts = [("Minimum file","*.min"),("Plain text file","*.txt"),("Other","*.*")]
        self.t_editor.tag_config("funs", foreground = "#00D413")
        self.t_editor.tag_config("pfuns", foreground = "#C48A00")
        self.t_editor.tag_config("synt", foreground = "#006CC4")
        self.t_editor.tag_config("str", foreground = "#777777")
        self.t_editor.tag_config("ufuns", foreground = "#FFAC1C")
        self.t_lines.tag_configure("right", justify='right')
        self.t_lines.tag_add("right", 1.0, "end")

        self.menu = Menu(self)
        self.config(menu=self.menu)
        self.filemenu = Menu(self.menu, tearoff = 0)
        self.filemenu.add_command(label = "Save As", command = self.save_as, accelerator = "Ctrl+Shift+S")
        self.filemenu.add_command(label = "Save", command = self.save, accelerator = "Ctrl+S")
        self.filemenu.add_command(label = "Open", command = self.open, accelerator = "Ctrl+O")
        self.filemenu.add_command(label = "New Window", command = self.new_window, accelerator = "Ctrl+N")
        self.menu.add_cascade(label = "File", menu = self.filemenu)
        self.his_menu = Menu(self.menu, tearoff = 0)
        self.his_menu.add_command(label = "Undo", command = self.his_left, accelerator="Ctrl+Z")
        self.his_menu.add_command(label = "Redo", command = self.his_right, accelerator="Ctrl+Shift+Z")
        #self.his_menu.add_command(label = "Flush", command = self.his_flush)
        self.menu.add_cascade(label = "History", menu = self.his_menu)
        self.run_menu = Menu(self.menu, tearoff = 0)
        self.run_menu.add_command(label = "Run", command = self.run, accelerator="F5")
        self.run_menu.add_command(label = "Change Interpreter", command = self.run_change)
        self.menu.add_cascade(label = "Run", menu = self.run_menu)

        self.master = master
        if master==None:
            self.mainloop()

    def new_window(self, event = None):
        parent = self.master if self.master else self
        GUIlet(parent)

    def run_change(self):
        self.interpreter = askopenfilename(filetypes = [("Windows executable","*.exe"),("Other","*.*")])

    def run(self, event = None):
        if self.interpreter:
            if self.fname:
                if os.name=="nt":#works with cmd
                    Process(target = worker, args = (f'{self.interpreter} "{self.fname}" & pause',)).start()
                else:#assumes you have bash
                    Process(target = worker, args = (f'{self.interpreter} "{self.fname}" && read -p "Press enter to exit"',)).start()
        else:
            self.interpreter = askopenfilename(filetypes = [("Windows executable","*.exe"),("Other","*.*")])
            self.run()

    def search(self, event = None):
        self.searcher.deiconify()
        self.searcher.entry.focus_set()

    def his_flush(self):
        self.history = zlib.compress(pickle.dumps([(self.t_editor.get("1.0","end-1c"),self.t_editor.index("insert"))]), 9)
        self.cancelled = []
        self.his_focus = 0

    def his_update(self, event):
        return
        #print(event.keycode, event.keysym)
        if self.his_focus<-1 and not event.keysym in ["Control_L","Shift_L","Z","z","Up","Left","Down","Right"]:
            old = pickle.loads(zlib.decompress(self.history))[:self.his_focus+1]
            self.history = zlib.compress(pickle.dumps(old), 9)
            self.his_focus = 0
            self.cancelled = []

    def his_left(self, event = None):
        #self.t_editor.edit_undo()
        self.highlight_manager2()
        return
        old = pickle.loads(zlib.decompress(self.history))
        self.cancelled.append((self.t_editor.get("1.0","end-1c"),self.t_editor.index("insert")))
        if self.his_focus>-len(old):
            ratio = self.t_editor.yview()
            self.his_focus-=1
            self.t_editor.delete("1.0",END)
            self.t_editor.insert("1.0", old[self.his_focus][0])
            #self.t_editor.tag_add("insert", old[self.his_focus][1], old[self.his_focus][1])
            self.t_editor.mark_set("insert", old[self.his_focus][1])
            self.highlight_manager2()

            self.t_editor.yview(MOVETO, ratio[0])
            self.t_lines.yview(MOVETO, ratio[0])
            #self.t_editor.see(old[self.his_focus][1])
            #self.t_lines.see(old[self.his_focus][1])

    def his_right(self, event = None):
        self.t_editor.edit_redo()
        self.highlight_manager2()
        return
        if self.cancelled:
            ratio = self.t_editor.yview()
            self.his_focus+=1
            text, line = self.cancelled.pop()
            self.t_editor.delete("1.0",END)
            self.t_editor.insert("1.0", text)
            self.t_editor.mark_set("insert", line)
            self.highlight_manager2()

            self.t_editor.yview(MOVETO, ratio[0])
            self.t_lines.yview(MOVETO, ratio[0])
            #self.t_editor.see(line)
            #self.t_lines.see(line)
        return
        old = pickle.loads(zlib.decompress(self.history))
        if self.his_focus<-1:
            self.his_focus+=1
            self.t_editor.delete("1.0",END)
            self.t_editor.insert("1.0", old[self.his_focus][0])
            self.t_editor.mark_set("insert", old[self.his_focus][1])#self.t_editor.tag_add("insert", old[self.his_focus][1], old[self.his_focus][1])
            self.highlight_manager2()

    def his_save(self, event = None):
        old = pickle.loads(zlib.decompress(self.history))
        self.his_focus = -1
        old.append(
            (self.t_editor.get("1.0","end-1c"), self.t_editor.index("insert"))
            )
        #print(old)
        self.history = zlib.compress(pickle.dumps(old), 9)
        self.his_focus = 0

    def remove_indent(self, event = None):
        f = int(self.t_editor.index("sel.first").split(".")[0])-1
        l = int(self.t_editor.index("sel.last").split(".")[0])-1
        lines = self.t_editor.get("1.0", "end-1c").split("\n")
        for n, line in enumerate(lines):
            if n<=l and n>=f:
                lines[n] = line[self.indent:] if line.startswith(" "*self.indent) else line
        self.t_editor.delete("1.0", END)
        self.t_editor.insert("1.0", "\n".join(lines))
        self.t_editor.tag_add("sel", f"{f+1}.0", f"{l+1}.end")
        self.t_editor.see(f"{f+1}.0")
        self.t_lines.see(f"{f+1}.0")
        self.highlight_manager2()

    def add_indent(self, event = None):
        f = int(self.t_editor.index("sel.first").split(".")[0])-1
        l = int(self.t_editor.index("sel.last").split(".")[0])-1
        lines = self.t_editor.get("1.0", "end-1c").split("\n")
        for n, line in enumerate(lines):
            if n<=l and n>=f:
                lines[n] = f"{' '*self.indent}{line}"
        self.t_editor.delete("1.0", "end")
        self.t_editor.insert("1.0", "\n".join(lines))
        self.t_editor.tag_add("sel", f"{f+1}.0", f"{l+1}.end")
        self.t_editor.see(f"{f+1}.0")
        self.t_lines.see(f"{f+1}.0")
        self.highlight_manager2()
        
    def scroll_left(self, event):
        ratio = self.t_editor.yview()
        self.t_lines.yview(MOVETO, ratio[0])#event_generate("<MouseWheel>", delta=event.delta, when="now")

    def highlight_ufuns(self):
        whole = self.t_editor.get("1.0", END)
        self.user_funs = []
        for line_n, line in enumerate(whole.split("\n")):
            if clean(line).startswith("func "):
                name = clean(line).split(" ")[1] if not clean(line).split(" ")[1].endswith(":") else clean(line).split(" ")[1][:-1]
                self.user_funs.append(name)

    def advanced_scroll1(self, event):
        if self.depth==0:
            self.depth+=1
            #self.t_lines.event_generate("<MouseWheel>", delta=event.delta, when="now")
            self.t_editor.event_generate("<MouseWheel>", delta=event.delta, when="now")
        else:
            self.depth = 0

    def advanced_scroll2(self, event):
        if self.depth==0:
            self.depth+=1
            self.t_lines.event_generate("<MouseWheel>", delta=event.delta, when="now")
            #self.t_editor.event_generate("<MouseWheel>", delta=event.delta, when="now")
        else:
            self.depth = 0

    def open(self, event = None):
        self.fname = askopenfilename(filetypes = self.exts)
        self.title(f"Minimum Editor [{self.fname}]")
        self.t_editor.delete("1.0", END)
        with open(self.fname, encoding = "utf-8") as f:
            self.t_editor.insert("1.0", f.read())
        self.history = zlib.compress(pickle.dumps([(self.t_editor.get("1.0","end-1c"), self.t_editor.index("insert"))]), 9)
        self.cancelled = []
        self.highlight_manager2()

    def save_as(self, event = None):
        self.fname = asksaveasfilename(filetypes = self.exts, defaultextension = "*.min")
        if self.fname:
            self.title(f"Minimum Editor [{self.fname}]")
            with open(self.fname, "w", encoding = "utf-8") as f:
                f.write(self.t_editor.get("1.0","end-1c"))

    def save(self, event = None):
        if not self.fname:
            self.save_as(event)
        else:
            with open(self.fname, "w", encoding = "utf-8") as f:
                f.write(self.t_editor.get("1.0","end-1c"))

    def multiple_yview(self, *args):
        self.t_lines.yview(*args)
        self.t_editor.yview(*args)

    def input_manager(self, event):
        ind = self.t_editor.index("insert")
        line, char = ind.split(".")
        curline = self.t_editor.get(f"{line}.0",f"{line}.end")
        #print(event.char, ord(event.char))
        if curline.endswith(":") and event.char==chr(13):
            counter = 0
            while curline[counter]==" ":
                counter+=1
            self.after(1, lambda: self.t_editor.insert(f"{int(line)+1}.0", (self.indent+counter)*" ") )
        elif event.char==chr(13):
            counter = 0
            while counter<len(curline) and curline[counter]==" ":
                counter+=1
            self.after(1, lambda: self.t_editor.insert(f"{int(line)+1}.0", counter*" ") )
        elif event.char==chr(8):
            if self.t_editor.get(f"{line}.0", ind).endswith(self.indent*" "):
                self.t_editor.delete(f"{line}.{int(char)-self.indent+1}", ind)
        elif event.char==chr(9):
            self.after(1, lambda: self.replace_tabs(ind))

        self.after(0, self.highlight_manager2)
        self.after(0, lambda e=event: self.scroll_left(e))

    def replace_tabs(self, pos):
        self.t_editor.delete(pos,f"{pos}+1c")
        self.t_editor.insert(pos, self.indent*" ")

    def highlight_manager2(self):
        [self.t_editor.tag_remove(tag, "1.0", END) for tag in ["pfuns","funs","synt","str","ufuns"]]
        self.t_lines.config(state = NORMAL)
        self.t_lines.delete("1.0", END)
        lines = pad([str(i+1) for i in range(self.t_editor.get("1.0",END).count("\n"))])
        self.t_lines.insert("1.0","\n".join(lines))
        self.t_lines.tag_add("right", 1.0, "end")
        self.t_lines.config(state = DISABLED)
        if self.wid!=len(lines[0]):
            self.wid = len(lines[0])
            self.t_lines.config(width = self.wid)

        self.highlight_ufuns()
        lines = self.t_editor.get("1.0","end-1c").splitlines()
        for line, string in enumerate(lines, 1):
            for tag, pattern in [("synt",f"\\b({'|'.join(self.synt)})\\b"),
                                 ("synt","\\+\\+"),
                                 ("synt","--"),
                                 ("synt","(<-|->|!)"),
                                 ("funs",f"\\b({'|'.join(self.funs)})\\b"),
                                 ("pfuns",f"\\b({'|'.join(self.pfuns)})\\b"),
                                 ("str",r'\"(\\.|[^\"])*\"'),
                                 ("str","#.*$"),
                                 ("ufuns", f"\\b({'|'.join(self.user_funs)})\\b")]:
                for obj in re.finditer(pattern, string):
                    start = f"{line}.{obj.start(0)}"
                    end = f"{line}.{obj.end(0)}"
                    self.t_editor.tag_add(tag, start, end)

    def highlight_manager(self):
        [self.t_editor.tag_remove(tag, "1.0", END) for tag in ["pfuns","funs","synt","str","ufuns"]]
        self.t_lines.delete("1.0", END)
        lines = pad([str(i+1) for i in range(self.t_editor.get("1.0",END).count("\n"))])
        self.t_lines.insert("1.0","\n".join(lines))
        self.t_lines.tag_add("right", 1.0, "end")

        for pf in self.pfuns:
            pf = pf+" "
            self.highlight(pf, "pfuns")

        for f in self.funs:
            self.highlight(f, "funs")

        for f in self.synt:
            self.highlight(f, "synt")

        self.highlight_ufuns()

        matches = []
        pattern = '"(.*?)"'
        text = self.t_editor.get("1.0", END).splitlines()
        for i, line in enumerate(text):
            for match in re.finditer(pattern, line):
                matches.append((f"{i + 1}.{match.start()}", f"{i + 1}.{match.end()}"))
            if "@" in '""'.join(line.split('"')[::2]):
                matches.append((f"{i + 1}.{line.find('@')}", f"{i + 1}.end"))
        for start, end in matches:
            self.t_editor.tag_add("str", start, end)

    def highlight(self, keyword, tag):
        """https://stackoverflow.com/questions/17829713/tkinter-highlight-colour-specific-lines-of-text-based-on-a-keyword"""
        pos = '1.0'
        while True:
            idx = self.t_editor.search(keyword, pos, END)
            if not idx:
                break
            pos = '{}+{}c'.format(idx, len(keyword))
            self.t_editor.tag_add(tag, idx, pos)

class GUIlet(GUI):
    def __init__(self, master):
        super().__init__()

def pad(linelist):
    maxlen = len(linelist[-1])
    return ["0"*(maxlen-len(f"{n}"))+f"{n}" for n in linelist]

if __name__=="__main__":
    freeze_support()
    gui = GUI()

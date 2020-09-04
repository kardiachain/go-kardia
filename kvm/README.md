# Kardia Virtual Machine (KVM)

KVMv0 is a mere lightweight version of EVM which supports majority of EVM opcodes, except for block operations (BFT DPoS vs. PoW)

Future KVM state will include performance enhancement and support for cross-chain operations.

### License
The EVM codes is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in this repository in the `LICENSE-3RD-PARTY.txt` file.


## Benchstat
* Installed [Benchstat](https://github.com/golang/perf)
* Quick guide for the usability of the tool:
<pre><code>$ go test -bench=. >old.txt
$ go test -bench=. >new.txt
$ benchstat old.txt new.txt
</code></pre>
* Example result:
<pre><code>name                              old time/op  new time/op  delta
JumpdestAnalysis_1200k-4          1.17ms ± 0%  0.77ms ± 9%   ~     (p=0.333 n=1+5)
JumpdestHashing_1200k-4           4.93ms ± 0%  6.39ms ±67%   ~     (p=1.000 n=1+5)
PrecompiledEcrecover/-Gas=3000-4   338µs ± 0%   296µs ±31%   ~     (p=0.667 n=1+5)
PrecompiledSha256/128-Gas=108-4    868ns ± 0%   885ns ±45%   ~     (p=1.000 n=1+5)
PrecompiledRipeMD/128-Gas=1080-4  2.23µs ± 0%  2.29µs ±44%   ~     (p=1.000 n=1+5)
PrecompiledIdentity/128-Gas=27-4  27.1ns ± 0%  21.3ns ±25%   ~     (p=0.333 n=1+5)
</code></pre>

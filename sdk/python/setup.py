from setuptools import setup, find_packages

setup(
    name="iploop",
    version="1.3.1",
    packages=find_packages(),
    install_requires=["requests>=2.28"],
    extras_require={"async": ["aiohttp>=3.8"]},
    author="IPLoop",
    author_email="partners@iploop.io",
    description="Residential proxy SDK — one-liner web fetching through millions of real IPs",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    url="https://iploop.io",
    python_requires=">=3.7",
    license="MIT",
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
    ],
)
